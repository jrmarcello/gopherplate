package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// TemplateModulePath is the original module path of the template project.
const TemplateModulePath = "github.com/jrmarcello/go-boilerplate"

// CleanupWiring regenerates server.go and router.go based on the scaffold config,
// removing references to disabled features. Must run AFTER RemoveDisabledFeatures.
func CleanupWiring(projectDir string, cfg Config) error {
	modulePath := detectModulePath(projectDir)

	data := wiringData{
		ModulePath:   modulePath,
		DB:           string(cfg.DB),
		KeepExamples: cfg.KeepExamples,
		Redis:        cfg.Redis,
		Idempotency:  cfg.Idempotency,
		Auth:         cfg.Auth,
	}

	if genServerErr := generateServerGo(projectDir, data); genServerErr != nil {
		return fmt.Errorf("generating server.go: %w", genServerErr)
	}

	if genRouterErr := generateRouterGo(projectDir, data); genRouterErr != nil {
		return fmt.Errorf("generating router.go: %w", genRouterErr)
	}

	return nil
}

type wiringData struct {
	ModulePath   string
	DB           string
	KeepExamples bool
	Redis        bool
	Idempotency  bool
	Auth         bool
}

func (d wiringData) DBDriverImport() string {
	switch DBDriver(d.DB) {
	case DBMySQL:
		return `_ "github.com/go-sql-driver/mysql"`
	case DBSQLite:
		return `_ "modernc.org/sqlite"`
	case DBOther:
		return `// TODO: add your database driver import here`
	default:
		return `_ "github.com/lib/pq"`
	}
}

func (d wiringData) DBDriverName() string {
	switch DBDriver(d.DB) {
	case DBMySQL:
		return "mysql"
	case DBSQLite:
		return "sqlite"
	case DBOther:
		return "postgres" // placeholder
	default:
		return "postgres"
	}
}

func detectModulePath(projectDir string) string {
	modPath := filepath.Join(projectDir, "go.mod")
	content, readErr := os.ReadFile(modPath) //nolint:gosec // CLI tool reads user-specified paths
	if readErr != nil {
		return TemplateModulePath
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimPrefix(line, "module ")
		}
	}

	return TemplateModulePath
}

func generateServerGo(projectDir string, data wiringData) error {
	tmpl, parseErr := template.New("server.go").Parse(serverGoTemplate)
	if parseErr != nil {
		return fmt.Errorf("parsing template: %w", parseErr)
	}

	outPath := filepath.Join(projectDir, "cmd", "api", "server.go")
	f, createErr := os.Create(outPath) //nolint:gosec // CLI tool writes to user-specified project directory
	if createErr != nil {
		return fmt.Errorf("creating file: %w", createErr)
	}
	defer func() { _ = f.Close() }()

	execErr := tmpl.Execute(f, data)
	if execErr != nil {
		return fmt.Errorf("executing template: %w", execErr)
	}

	return nil
}

func generateRouterGo(projectDir string, data wiringData) error {
	tmpl, parseErr := template.New("router.go").Parse(routerGoTemplate)
	if parseErr != nil {
		return fmt.Errorf("parsing template: %w", parseErr)
	}

	outPath := filepath.Join(projectDir, "internal", "infrastructure", "web", "router", "router.go")
	f, createErr := os.Create(outPath)
	if createErr != nil {
		return fmt.Errorf("creating file: %w", createErr)
	}
	defer func() { _ = f.Close() }()

	execErr := tmpl.Execute(f, data)
	if execErr != nil {
		return fmt.Errorf("executing template: %w", execErr)
	}

	return nil
}

//nolint:lll
const serverGoTemplate = `package main

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

	"{{.ModulePath}}/config"
{{- if .KeepExamples}}
	docs "{{.ModulePath}}/docs"
	"{{.ModulePath}}/internal/infrastructure/db/postgres/repository"
	infratelemetry "{{.ModulePath}}/internal/infrastructure/telemetry"
	"{{.ModulePath}}/internal/infrastructure/web/handler"
{{- end}}
	"{{.ModulePath}}/internal/infrastructure/web/router"
{{- if .KeepExamples}}
	roleuc "{{.ModulePath}}/internal/usecases/role"
	useruc "{{.ModulePath}}/internal/usecases/user"
{{- end}}
{{- if .Redis}}
	"{{.ModulePath}}/pkg/cache"
	"{{.ModulePath}}/pkg/cache/redisclient"
{{- end}}
	"{{.ModulePath}}/pkg/database"
	"{{.ModulePath}}/pkg/health"
{{- if .Idempotency}}
	"{{.ModulePath}}/pkg/idempotency"
	"{{.ModulePath}}/pkg/idempotency/redisstore"
{{- end}}
	"{{.ModulePath}}/pkg/logutil"
	pkgtelemetry "{{.ModulePath}}/pkg/telemetry"
	"{{.ModulePath}}/pkg/telemetry/otelgrpc"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	{{.DBDriverImport}}
{{- if .KeepExamples}}
	"go.opentelemetry.io/otel"
{{- end}}
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
		Driver:          "{{.DBDriverName}}",
		DSN:             cfg.DB.GetWriterDSN(),
		MaxOpenConns:    cfg.DB.MaxOpenConns,
		MaxIdleConns:    cfg.DB.MaxIdleConns,
		ConnMaxLifetime: cfg.DB.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.DB.ConnMaxIdleTime,
	}

	var readerCfg *database.Config
	if cfg.DB.ReplicaEnabled {
		readerCfg = &database.Config{
			Driver:          "{{.DBDriverName}}",
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

	// Wrap stdlib connections for sqlx-based repositories
	sqlxWriter := sqlx.NewDb(cluster.Writer(), "{{.DBDriverName}}")
	sqlxReader := sqlx.NewDb(cluster.Reader(), "{{.DBDriverName}}")

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
{{if .KeepExamples}}
	// 5. Business Metrics (injected into handlers, not global)
	businessMetrics, metricsErr := infratelemetry.NewMetrics(otel.Meter(cfg.Otel.ServiceName))
	if metricsErr != nil {
		slog.Warn("Failed to create business metrics", "error", metricsErr)
	}
{{end}}
	// 6. Dependencies (Dependency Injection)
	var httpMetrics *pkgtelemetry.HTTPMetrics
	if tp != nil {
		httpMetrics = tp.HTTPMetrics()
	}
	deps := buildDependencies(cluster, sqlxWriter, sqlxReader, cfg, httpMetrics{{if .KeepExamples}}, businessMetrics{{end}})
{{if .KeepExamples}}
	// Swagger Dynamic Config
	if cfg.Swagger.Host != "" {
		docs.SwaggerInfo.Host = cfg.Swagger.Host
	} else {
		docs.SwaggerInfo.Host = "localhost:" + cfg.Server.Port
	}
{{end}}
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

func buildDependencies(cluster *database.DBCluster, sqlxWriter, sqlxReader *sqlx.DB, cfg *config.Config, httpMetrics *pkgtelemetry.HTTPMetrics{{if .KeepExamples}}, businessMetrics *infratelemetry.Metrics{{end}}) router.Dependencies {
{{- if .KeepExamples}}
	// Repositories (sqlx wrappers over stdlib *sql.DB connections)
	repo := repository.NewUserRepository(sqlxWriter, sqlxReader)
{{- end}}
{{if .Redis}}
	// Cache (optional)
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
{{end}}
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
{{- if .Redis}}
	if redisClient != nil && redisClient.UnderlyingClient() != nil {
		checker.Register("redis", false, func(ctx context.Context) error {
			return redisClient.Ping(ctx)
		})
	}
{{- end}}
{{if .Redis}}
	// Singleflight protection (prevents cache stampede on concurrent reads)
	flightGroup := cache.NewFlightGroup()
{{end}}
{{- if .KeepExamples}}
	// Use Cases (with optional cache via builder pattern)
	createUC := useruc.NewCreateUseCase(repo)
	getUC := useruc.NewGetUseCase(repo){{if .Redis}}.WithCache(redisClient).WithFlight(flightGroup){{end}}
	listUC := useruc.NewListUseCase(repo)
	updateUC := useruc.NewUpdateUseCase(repo){{if .Redis}}.WithCache(redisClient){{end}}
	deleteUC := useruc.NewDeleteUseCase(repo){{if .Redis}}.WithCache(redisClient){{end}}
{{end}}
{{- if .Idempotency}}
	// Idempotency Store (optional — uses Redis when enabled)
	var idempotencyStore idempotency.Store
	if cfg.Idempotency.Enabled {
		if rc := redisClient.UnderlyingClient(); rc != nil {
			ttl, _ := time.ParseDuration(cfg.Idempotency.TTL)
			lockTTL, _ := time.ParseDuration(cfg.Idempotency.LockTTL)
			idempotencyStore = redisstore.NewRedisStore(rc, ttl, lockTTL)
		}
	}
{{end}}
{{- if .KeepExamples}}
	// --- Role Domain (simpler, no cache/singleflight) ---
	roleRepo := repository.NewRoleRepository(sqlxWriter, sqlxReader)
	roleCreateUC := roleuc.NewCreateUseCase(roleRepo)
	roleListUC := roleuc.NewListUseCase(roleRepo)
	roleDeleteUC := roleuc.NewDeleteUseCase(roleRepo)
	roleHandler := handler.NewRoleHandler(roleCreateUC, roleListUC, roleDeleteUC)

	// --- Handlers ---
	userHandler := handler.NewUserHandler(createUC, getUC, listUC, updateUC, deleteUC, businessMetrics)
{{end}}
	return router.Dependencies{
		HealthChecker: checker,
{{- if .KeepExamples}}
		UserHandler:  userHandler,
		RoleHandler:  roleHandler,
{{- end}}
		HTTPMetrics: httpMetrics,
{{- if .Idempotency}}
		IdempotencyStore: idempotencyStore,
{{- end}}
		Config: router.Config{
			ServiceName:    cfg.Otel.ServiceName,
{{- if .Auth}}
			ServiceKeysEnabled: cfg.Auth.Enabled,
			ServiceKeys:        cfg.Auth.ServiceKeys,
{{- end}}
			SwaggerEnabled: cfg.Swagger.Enabled,
		},
	}
}

func newServer(port string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              ":" + port,
		Handler:           h,
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
`

//nolint:lll
const routerGoTemplate = `package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
{{- if .KeepExamples}}
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
{{- end}}
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

{{- if .KeepExamples}}
	"{{.ModulePath}}/internal/infrastructure/web/handler"
{{- end}}
	"{{.ModulePath}}/internal/infrastructure/web/middleware"
	"{{.ModulePath}}/pkg/health"
	"{{.ModulePath}}/pkg/httputil/httpgin"
{{- if .Idempotency}}
	"{{.ModulePath}}/pkg/idempotency"
{{- end}}
	"{{.ModulePath}}/pkg/telemetry"
)

// Config contém configurações do router
type Config struct {
	ServiceName string
{{- if .Auth}}
	ServiceKeysEnabled bool   // fail-closed em HML/PRD se keys vazio
	ServiceKeys        string // "service1:key1,service2:key2"
{{- end}}
	SwaggerEnabled bool
}

// Dependencies agrupa todas as dependências necessárias para o router
type Dependencies struct {
	HealthChecker *health.Checker
{{- if .KeepExamples}}
	UserHandler   *handler.UserHandler
	RoleHandler   *handler.RoleHandler
{{- end}}
	HTTPMetrics *telemetry.HTTPMetrics
{{- if .Idempotency}}
	IdempotencyStore idempotency.Store
{{- end}}
	Config Config
}

// Setup configura e retorna o router Gin com todos os middlewares e rotas
func Setup(deps Dependencies) *gin.Engine {
	r := gin.New()

	// Recovery middleware (panic recovery)
	r.Use(gin.Recovery())

	// OpenTelemetry (must be before Logger to populate trace_id)
	r.Use(otelgin.Middleware(deps.Config.ServiceName))

	// HTTP Metrics (count, duration, Apdex)
	r.Use(middleware.Metrics(deps.HTTPMetrics))

	// Custom structured logger
	r.Use(middleware.Logger())
{{if .Idempotency}}
	// Idempotency (optional — only if store is provided)
	if deps.IdempotencyStore != nil {
		r.Use(middleware.Idempotency(deps.IdempotencyStore))
	}
{{end}}
	// Public routes (no auth required)
	if deps.Config.SwaggerEnabled {
		registerSwaggerRoutes(r)
	}
	registerHealthRoutes(r, deps)
{{if .Auth}}
	// Protected routes (auth required if SERVICE_KEYS is configured)
	authConfig := middleware.ServiceKeyConfig{
		Enabled: deps.Config.ServiceKeysEnabled,
		Keys:    middleware.ParseServiceKeys(deps.Config.ServiceKeys),
	}
	protected := r.Group("")
	protected.Use(middleware.ServiceKeyAuth(authConfig))
{{- if .KeepExamples}}
	RegisterUserRoutes(protected, deps.UserHandler)
	RegisterRoleRoutes(protected, deps.RoleHandler)
{{- end}}
{{- else}}
{{- if .KeepExamples}}
	// API routes
	api := r.Group("")
	RegisterUserRoutes(api, deps.UserHandler)
	RegisterRoleRoutes(api, deps.RoleHandler)
{{- end}}
{{- end}}

	return r
}

// registerSwaggerRoutes registra rotas do Swagger
func registerSwaggerRoutes(r *gin.Engine) {
{{- if .KeepExamples}}
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
{{- else}}
	// TODO: uncomment after running swag init
	// r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	_ = r
{{- end}}
}

// registerHealthRoutes registra rotas de health check
func registerHealthRoutes(r *gin.Engine, deps Dependencies) {
	// Liveness - always ok (K8s restart if process is dead)
	r.GET("/health", func(c *gin.Context) {
		httpgin.SendSuccess(c, http.StatusOK, gin.H{
			"status":  "ok",
			"service": deps.Config.ServiceName,
		})
	})

	// Readiness - checks all dependencies
	r.GET("/ready", func(c *gin.Context) {
		healthy, statuses := deps.HealthChecker.RunAll(c.Request.Context())

		result := gin.H{
			"status": "ready",
		}
		if !healthy {
			result["status"] = "not ready"
		}
		result["checks"] = statuses

		status := http.StatusOK
		if !healthy {
			status = http.StatusServiceUnavailable
		}
		httpgin.SendSuccess(c, status, result)
	})
}
`
