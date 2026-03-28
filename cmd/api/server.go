package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bitbucket.org/appmax-space/go-boilerplate/config"
	docs "bitbucket.org/appmax-space/go-boilerplate/docs"
	"bitbucket.org/appmax-space/go-boilerplate/internal/infrastructure/db/postgres/repository"
	infratelemetry "bitbucket.org/appmax-space/go-boilerplate/internal/infrastructure/telemetry"
	"bitbucket.org/appmax-space/go-boilerplate/internal/infrastructure/web/handler"
	"bitbucket.org/appmax-space/go-boilerplate/internal/infrastructure/web/router"
	entityuc "bitbucket.org/appmax-space/go-boilerplate/internal/usecases/entity_example"
	pkgcache "bitbucket.org/appmax-space/go-boilerplate/pkg/cache"
	"bitbucket.org/appmax-space/go-boilerplate/pkg/database"
	"bitbucket.org/appmax-space/go-boilerplate/pkg/health"
	"bitbucket.org/appmax-space/go-boilerplate/pkg/idempotency"
	"bitbucket.org/appmax-space/go-boilerplate/pkg/logutil"
	pkgtelemetry "bitbucket.org/appmax-space/go-boilerplate/pkg/telemetry"
	"go.opentelemetry.io/otel"
)

// Start initializes the application following the composition pattern:
// Config → Logger → Telemetry → Database → Dependencies → Router → Server
func Start(ctx context.Context, cfg *config.Config) error {
	// 1. Logger
	logger := setupLogger()
	slog.SetDefault(logger)

	// 2. Telemetry (OpenTelemetry Traces + Metrics)
	tp, tpErr := pkgtelemetry.Setup(ctx, pkgtelemetry.Config{
		ServiceName:  cfg.Otel.ServiceName,
		CollectorURL: cfg.Otel.CollectorURL,
		Enabled:      cfg.Otel.CollectorURL != "",
		Insecure:     cfg.Otel.Insecure,
	})
	if tpErr != nil {
		return tpErr
	}
	defer shutdownTelemetry(tp, logger)

	// 3. Database (Writer/Reader Cluster)
	writerCfg := database.Config{
		DSN:             cfg.DB.GetWriterDSN(),
		MaxOpenConns:    cfg.DB.MaxOpenConns,
		MaxIdleConns:    cfg.DB.MaxIdleConns,
		ConnMaxLifetime: cfg.DB.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.DB.ConnMaxIdleTime,
	}

	var readerCfg *database.Config
	if cfg.DB.ReplicaEnabled {
		readerCfg = &database.Config{
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
	defer cluster.Close()

	// SSL mode warning for non-development environments
	if cfg.DB.SSLMode == "disable" && cfg.Server.Env != "development" {
		slog.Warn("database connection using sslmode=disable in non-development environment")
	}

	// 4. Register DB Pool Metrics
	if regErr := pkgtelemetry.RegisterDBPoolMetrics(ctx, cfg.Otel.ServiceName, cluster.Writer().DB, "writer"); regErr != nil {
		slog.Warn("Failed to register DB pool metrics", "error", regErr)
	}

	if cluster.HasSeparateReader() {
		if regErr := pkgtelemetry.RegisterDBPoolMetrics(ctx, cfg.Otel.ServiceName, cluster.Reader().DB, "reader"); regErr != nil {
			slog.Warn("Failed to register reader DB pool metrics", "error", regErr)
		}
	}

	// 5. Business Metrics (injected into handlers, not global)
	businessMetrics, metricsErr := infratelemetry.NewMetrics(otel.Meter(cfg.Otel.ServiceName))
	if metricsErr != nil {
		slog.Warn("Failed to create business metrics", "error", metricsErr)
	}

	// 6. Dependencies (Dependency Injection)
	deps := buildDependencies(cluster, cfg, tp.HTTPMetrics(), businessMetrics)

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
	return slog.New(logutil.NewFanoutHandler(stdout))
}

func shutdownTelemetry(tp *pkgtelemetry.Provider, logger *slog.Logger) {
	if shutdownErr := tp.Shutdown(context.Background()); shutdownErr != nil {
		logger.Error("failed to shutdown telemetry", "error", shutdownErr)
	}
}

func buildDependencies(cluster *database.DBCluster, cfg *config.Config, httpMetrics *pkgtelemetry.HTTPMetrics, businessMetrics *infratelemetry.Metrics) router.Dependencies {
	// Repositories (cluster handles Writer/Reader routing internally)
	repo := repository.NewEntityRepository(cluster)

	// Cache (optional)
	redisClient, cacheErr := pkgcache.NewRedisClient(pkgcache.RedisConfig{
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

	// Health Checker
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

	// Singleflight protection (prevents cache stampede on concurrent reads)
	flightGroup := pkgcache.NewFlightGroup()

	// Use Cases (with optional cache via builder pattern)
	createUC := entityuc.NewCreateUseCase(repo)
	getUC := entityuc.NewGetUseCase(repo).WithCache(redisClient).WithFlight(flightGroup)
	listUC := entityuc.NewListUseCase(repo)
	updateUC := entityuc.NewUpdateUseCase(repo).WithCache(redisClient)
	deleteUC := entityuc.NewDeleteUseCase(repo).WithCache(redisClient)

	// Idempotency Store (optional — uses Redis if available)
	var idempotencyStore idempotency.Store
	if rc := redisClient.UnderlyingClient(); rc != nil {
		idempotencyStore = idempotency.NewRedisStore(rc, 24*time.Hour, 30*time.Second)
	}

	// Handlers
	entityHandler := handler.NewEntityHandler(createUC, getUC, listUC, updateUC, deleteUC, businessMetrics)

	return router.Dependencies{
		HealthChecker:    checker,
		EntityHandler:    entityHandler,
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
