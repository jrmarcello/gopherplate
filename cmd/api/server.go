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

	"github.com/jmoiron/sqlx"

	"bitbucket.org/appmax-space/go-boilerplate/config"
	docs "bitbucket.org/appmax-space/go-boilerplate/docs"
	"bitbucket.org/appmax-space/go-boilerplate/internal/infrastructure/cache"
	"bitbucket.org/appmax-space/go-boilerplate/internal/infrastructure/db/postgres"
	"bitbucket.org/appmax-space/go-boilerplate/internal/infrastructure/db/postgres/repository"
	"bitbucket.org/appmax-space/go-boilerplate/internal/infrastructure/telemetry"
	"bitbucket.org/appmax-space/go-boilerplate/internal/infrastructure/web/handler"
	"bitbucket.org/appmax-space/go-boilerplate/internal/infrastructure/web/router"
	entityuc "bitbucket.org/appmax-space/go-boilerplate/internal/usecases/entity"
)

// Start inicializa a aplicação seguindo o padrão de composição:
// Config → Logger → Telemetry → Database → Dependencies → Router → Server
func Start(ctx context.Context, cfg *config.Config) error {
	// 1. Logger
	logger := setupLogger()
	slog.SetDefault(logger)

	// 2. Telemetry (OpenTelemetry Traces + Metrics)
	tp, err := telemetry.Setup(ctx, telemetry.Config{
		ServiceName:  cfg.Otel.ServiceName,
		CollectorURL: cfg.Otel.CollectorURL,
		Enabled:      cfg.Otel.CollectorURL != "",
	})
	if err != nil {
		return err
	}
	defer shutdownTelemetry(tp, logger)

	// 3. Database
	conn, err := postgres.NewPostgres(cfg.DB.DSN)
	if err != nil {
		return err
	}
	defer conn.Close()

	// 4. Dependencies (Dependency Injection)
	deps := buildDependencies(conn, cfg)

	// Swagger Dynamic Config
	docs.SwaggerInfo.Host = "localhost:" + cfg.Server.Port

	// 5. Router
	r := router.Setup(deps)

	// 6. Server
	srv := newServer(cfg.Server.Port, r)

	// 7. Graceful Shutdown
	return runWithGracefulShutdown(srv, logger)
}

func setupLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, nil))
}

func shutdownTelemetry(tp *telemetry.Provider, logger *slog.Logger) {
	if err := tp.Shutdown(context.Background()); err != nil {
		logger.Error("failed to shutdown telemetry", "error", err)
	}
}

func buildDependencies(conn *sqlx.DB, cfg *config.Config) router.Dependencies {
	// Repositories
	repo := &repository.EntityRepository{DB: conn}

	// Cache (optional)
	redisClient, err := cache.NewRedisClient(cache.Config{
		URL:     cfg.Redis.URL,
		TTL:     cfg.Redis.TTL,
		Enabled: cfg.Redis.Enabled,
	})
	if err != nil {
		slog.Warn("Redis cache disabled", "error", err)
	}

	// Use Cases (with optional cache)
	createUC := entityuc.NewCreateUseCase(repo)
	getUC := entityuc.NewGetUseCase(repo, redisClient)
	listUC := entityuc.NewListUseCase(repo)
	updateUC := entityuc.NewUpdateUseCase(repo, redisClient)
	deleteUC := entityuc.NewDeleteUseCase(repo, redisClient)

	// Handlers
	entityHandler := handler.NewEntityHandler(createUC, getUC, listUC, updateUC, deleteUC)

	return router.Dependencies{
		DB:            conn,
		EntityHandler: entityHandler,
		Config: router.Config{
			ServiceName: cfg.Otel.ServiceName,
			ServiceKeys: cfg.Auth.ServiceKeys,
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
	// Start server in goroutine
	go func() {
		logger.Info("Starting server", "port", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}

	logger.Info("server exited properly")
	return nil
}
