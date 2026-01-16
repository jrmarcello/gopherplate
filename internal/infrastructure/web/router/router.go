package router

import (
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	"bitbucket.org/appmax-space/go-boilerplate/internal/infrastructure/web/handler"
	"bitbucket.org/appmax-space/go-boilerplate/internal/infrastructure/web/middleware"
)

// Config contém configurações do router
type Config struct {
	ServiceName string
	ServiceKeys string // "service1:key1,service2:key2"
}

// Dependencies agrupa todas as dependências necessárias para o router
type Dependencies struct {
	DB            *sqlx.DB
	EntityHandler *handler.EntityHandler
	Config        Config
}

// Setup configura e retorna o router Gin com todos os middlewares e rotas
func Setup(deps Dependencies) *gin.Engine {
	r := gin.New()

	// Recovery middleware (panic recovery)
	r.Use(gin.Recovery())

	// OpenTelemetry (must be before Logger to populate trace_id)
	r.Use(otelgin.Middleware(deps.Config.ServiceName))

	// Custom structured logger
	r.Use(middleware.Logger())

	// CORS
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID", "X-Idempotency-Key", "X-Service-Name", "X-Service-Key"},
		ExposeHeaders:    []string{"Content-Length", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Idempotency
	r.Use(middleware.Idempotency(middleware.DefaultIdempotencyConfig()))

	// Public routes (no auth required)
	registerSwaggerRoutes(r)
	registerHealthRoutes(r, deps)

	// Protected routes (auth required if SERVICE_KEYS is configured)
	authConfig := middleware.ServiceKeyConfig{
		Keys: middleware.ParseServiceKeys(deps.Config.ServiceKeys),
	}
	protected := r.Group("")
	protected.Use(middleware.ServiceKeyAuth(authConfig))
	RegisterEntityRoutes(protected, deps.EntityHandler)

	return r
}

// registerSwaggerRoutes registra rotas do Swagger
func registerSwaggerRoutes(r *gin.Engine) {
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}

// registerHealthRoutes registra rotas de health check
func registerHealthRoutes(r *gin.Engine, deps Dependencies) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": deps.Config.ServiceName,
		})
	})

	r.GET("/ready", func(c *gin.Context) {
		if err := deps.DB.Ping(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not ready",
				"error":  "database connection failed",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
}
