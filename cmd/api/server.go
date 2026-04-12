package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/jrmarcello/gopherplate/config"
	docs "github.com/jrmarcello/gopherplate/docs"
	"github.com/jrmarcello/gopherplate/internal/bootstrap"
	infratelemetry "github.com/jrmarcello/gopherplate/internal/infrastructure/telemetry"
	"github.com/jrmarcello/gopherplate/internal/infrastructure/web/router"
	"github.com/jrmarcello/gopherplate/pkg/cache/redisclient"
	"github.com/jrmarcello/gopherplate/pkg/database"
	"github.com/jrmarcello/gopherplate/pkg/health"
	"github.com/jrmarcello/gopherplate/pkg/idempotency"
	"github.com/jrmarcello/gopherplate/pkg/idempotency/redisstore"
	"github.com/jrmarcello/gopherplate/pkg/logutil"
	pkgtelemetry "github.com/jrmarcello/gopherplate/pkg/telemetry"
	"github.com/jrmarcello/gopherplate/pkg/telemetry/otelgrpc"
	_ "github.com/lib/pq"
	"go.opentelemetry.io/otel"
)

// Start initializes the application following the composition pattern:
// Config → Logger → Telemetry → Database → Dependencies → Router → Server
func Start(ctx context.Context, cfg *config.Config) error {
	// 0. Validate config
	if validateErr := cfg.Validate(); validateErr != nil {
		return fmt.Errorf("invalid configuration: %w", validateErr)
	}

	// 1. Logger
	logger := setupLogger()
	slog.SetDefault(logger)

	// Set Gin mode from config (avoid "Running in debug mode" warning in production)
	if cfg.Server.GinMode != "" {
		gin.SetMode(cfg.Server.GinMode)
	}

	// 2. Telemetry (OpenTelemetry Traces + Metrics)
	// Graceful degradation: if OTel setup fails, app continues without telemetry.
	var exporterOpts []pkgtelemetry.Option
	if cfg.Otel.CollectorURL != "" {
		grpcOpts, exporterErr := otelgrpc.Exporters(ctx, otelgrpc.Config{
			CollectorURL: cfg.Otel.CollectorURL,
			Insecure:     cfg.Otel.Insecure,
		})
		if exporterErr != nil {
			slog.Warn("Telemetry exporter setup failed, continuing without observability", "error", exporterErr)
		} else {
			exporterOpts = grpcOpts
		}
	}

	tp, tpErr := pkgtelemetry.Setup(ctx, pkgtelemetry.Config{
		ServiceName: cfg.Otel.ServiceName,
		Enabled:     cfg.Otel.CollectorURL != "",
	}, exporterOpts...)
	if tpErr != nil {
		slog.Warn("Telemetry setup failed, continuing without observability", "error", tpErr)
	}
	if tp != nil {
		defer shutdownTelemetry(tp, logger)
	}

	// 3. Database (Writer/Reader Cluster)
	writerCfg := database.Config{
		Driver:          "postgres",
		DSN:             cfg.DB.GetWriterDSN(),
		MaxOpenConns:    cfg.DB.MaxOpenConns,
		MaxIdleConns:    cfg.DB.MaxIdleConns,
		ConnMaxLifetime: cfg.DB.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.DB.ConnMaxIdleTime,
	}

	var readerCfg *database.Config
	if cfg.DB.ReplicaEnabled {
		readerCfg = &database.Config{
			Driver:          "postgres",
			DSN:             cfg.DB.GetReaderDSN(),
			MaxOpenConns:    cfg.DB.ReplicaMaxOpenConns,
			MaxIdleConns:    cfg.DB.ReplicaMaxIdleConns,
			ConnMaxLifetime: cfg.DB.ReplicaConnMaxLifetime,
			ConnMaxIdleTime: cfg.DB.ReplicaConnMaxIdleTime,
		}
	}

	cluster, clusterErr := database.NewDBCluster(writerCfg, readerCfg)
	if clusterErr != nil {
		return clusterErr
	}
	defer func() { _ = cluster.Close() }()

	// Wrap stdlib connections for sqlx-based repositories
	sqlxWriter := sqlx.NewDb(cluster.Writer(), "postgres")
	sqlxReader := sqlx.NewDb(cluster.Reader(), "postgres")

	// SSL mode warning for non-development environments
	if cfg.DB.SSLMode == "disable" && cfg.Server.Env != "development" {
		slog.Warn("database connection using sslmode=disable in non-development environment")
	}

	// 4. Register DB Pool Metrics
	if regErr := pkgtelemetry.RegisterDBPoolMetrics(ctx, cfg.Otel.ServiceName, cluster.Writer(), "writer"); regErr != nil {
		slog.Warn("Failed to register DB pool metrics", "error", regErr)
	}

	if cluster.HasSeparateReader() {
		if regErr := pkgtelemetry.RegisterDBPoolMetrics(ctx, cfg.Otel.ServiceName, cluster.Reader(), "reader"); regErr != nil {
			slog.Warn("Failed to register reader DB pool metrics", "error", regErr)
		}
	}

	// 5. Business Metrics (injected into handlers, not global)
	businessMetrics, metricsErr := infratelemetry.NewMetrics(otel.Meter(cfg.Otel.ServiceName))
	if metricsErr != nil {
		slog.Warn("Failed to create business metrics", "error", metricsErr)
	}

	// 6. Dependencies (Dependency Injection)
	var httpMetrics *pkgtelemetry.HTTPMetrics
	if tp != nil {
		httpMetrics = tp.HTTPMetrics()
	}
	deps := buildDependencies(cluster, sqlxWriter, sqlxReader, cfg, httpMetrics, businessMetrics)

	// Swagger Dynamic Config
	if cfg.Swagger.Host != "" {
		docs.SwaggerInfo.Host = cfg.Swagger.Host
	} else {
		docs.SwaggerInfo.Host = "localhost:" + cfg.Server.Port
	}

	// 7. Router
	r := router.Setup(deps)

	// 8. Server
	srv := newServer(cfg.Server.Port, r)

	// 9. Graceful Shutdown
	return runWithGracefulShutdown(srv, logger)
}

func setupLogger() *slog.Logger {
	stdout := slog.NewJSONHandler(os.Stdout, nil)
	masked := logutil.NewMaskingHandler(logutil.NewMasker(logutil.DefaultBRConfig()), stdout)
	return slog.New(logutil.NewFanoutHandler(masked))
}

func shutdownTelemetry(tp *pkgtelemetry.Provider, logger *slog.Logger) {
	if shutdownErr := tp.Shutdown(context.Background()); shutdownErr != nil {
		logger.Error("failed to shutdown telemetry", "error", shutdownErr)
	}
}

func buildDependencies(cluster *database.DBCluster, sqlxWriter, sqlxReader *sqlx.DB, cfg *config.Config, httpMetrics *pkgtelemetry.HTTPMetrics, businessMetrics *infratelemetry.Metrics) router.Dependencies {
	// Cache (optional — config-dependent, stays in server.go)
	redisClient, cacheErr := redisclient.NewRedisClient(redisclient.RedisConfig{
		URL:          cfg.Redis.URL,
		TTL:          cfg.Redis.TTL,
		Enabled:      cfg.Redis.Enabled,
		PoolSize:     cfg.Redis.PoolSize,
		MinIdleConns: cfg.Redis.MinIdleConns,
		DialTimeout:  cfg.Redis.DialTimeout,
		ReadTimeout:  cfg.Redis.ReadTimeout,
		WriteTimeout: cfg.Redis.WriteTimeout,
	})
	if cacheErr != nil {
		slog.Warn("Redis cache disabled", "error", cacheErr)
	}

	// Health Checker (cross-cutting, stays in server.go)
	checker := health.New()
	checker.Register("database_writer", true, func(ctx context.Context) error {
		return cluster.Writer().PingContext(ctx)
	})
	if cluster.HasSeparateReader() {
		checker.Register("database_reader", false, func(ctx context.Context) error {
			return cluster.Reader().PingContext(ctx)
		})
	}
	if redisClient != nil && redisClient.UnderlyingClient() != nil {
		checker.Register("redis", false, func(ctx context.Context) error {
			return redisClient.Ping(ctx)
		})
	}

	// Bootstrap container (repos, use cases, handlers for all domains)
	c := bootstrap.New(sqlxWriter, sqlxReader, redisClient, businessMetrics)

	// Idempotency Store (optional — config-dependent, stays in server.go)
	var idempotencyStore idempotency.Store
	if cfg.Idempotency.Enabled {
		if rc := redisClient.UnderlyingClient(); rc != nil {
			ttl, _ := time.ParseDuration(cfg.Idempotency.TTL)
			lockTTL, _ := time.ParseDuration(cfg.Idempotency.LockTTL)
			idempotencyStore = redisstore.NewRedisStore(rc, ttl, lockTTL)
		}
	}

	return router.Dependencies{
		HealthChecker:    checker,
		UserHandler:      c.Handlers.User,
		RoleHandler:      c.Handlers.Role,
		HTTPMetrics:      httpMetrics,
		IdempotencyStore: idempotencyStore,
		Config: router.Config{
			ServiceName:        cfg.Otel.ServiceName,
			ServiceKeysEnabled: cfg.Auth.Enabled,
			ServiceKeys:        cfg.Auth.ServiceKeys,
			SwaggerEnabled:     cfg.Swagger.Enabled,
		},
	}
}

func newServer(port string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB — protects against oversized headers
	}
}

func runWithGracefulShutdown(srv *http.Server, logger *slog.Logger) error {
	// Error channel to capture server startup failures without os.Exit in goroutine
	errCh := make(chan error, 1)
	go func() {
		logger.Info("Starting server", "port", srv.Addr)
		if listenErr := srv.ListenAndServe(); listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			errCh <- listenErr
		}
	}()

	// Wait for interrupt signal or server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case listenErr := <-errCh:
		return listenErr
	case <-quit:
		// proceed to graceful shutdown
	}

	logger.Info("shutting down server...")

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if shutdownErr := srv.Shutdown(shutdownCtx); shutdownErr != nil {
		return shutdownErr
	}

	logger.Info("server exited properly")
	return nil
}
