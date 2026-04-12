package bootstrap

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	"github.com/jrmarcello/gopherplate/internal/infrastructure/web/middleware"
	"github.com/jrmarcello/gopherplate/internal/infrastructure/web/router"
	"github.com/jrmarcello/gopherplate/pkg/cache"
	"github.com/jrmarcello/gopherplate/pkg/httputil/httpgin"
)

// NewForTest creates a Container suitable for testing. It uses the same DB
// connection as both writer and reader, and passes nil metrics.
func NewForTest(t testing.TB, db *sqlx.DB, cacheClient cache.Cache) *Container {
	t.Helper()
	return New(db, db, cacheClient, nil)
}

// SetupTestRouter creates a gin.Engine in test mode with all routes (user and
// role) registered, CustomRecovery middleware, health endpoints, and a panic-test
// route. No auth middleware is applied.
func SetupTestRouter(t testing.TB, db *sqlx.DB, cacheClient cache.Cache) *gin.Engine {
	t.Helper()

	c := NewForTest(t, db, cacheClient)
	r := newTestEngine()

	registerTestHealthRoutes(r, db)

	// Register all domain routes without auth
	group := r.Group("")
	router.RegisterUserRoutes(group, c.Handlers.User)
	router.RegisterRoleRoutes(group, c.Handlers.Role)

	return r
}

// SetupTestRouterWithAuth creates a gin.Engine in test mode with all routes
// (user and role) registered behind service key authentication middleware.
// serviceKeys uses the format "service1:key1,service2:key2".
func SetupTestRouterWithAuth(t testing.TB, db *sqlx.DB, cacheClient cache.Cache, serviceKeys string) *gin.Engine {
	t.Helper()

	c := NewForTest(t, db, cacheClient)
	r := newTestEngine()

	registerTestHealthRoutes(r, db)

	// Register all domain routes behind auth middleware
	authConfig := middleware.ServiceKeyConfig{
		Enabled: true,
		Keys:    middleware.ParseServiceKeys(serviceKeys),
	}
	protected := r.Group("")
	protected.Use(middleware.ServiceKeyAuth(authConfig))
	router.RegisterUserRoutes(protected, c.Handlers.User)
	router.RegisterRoleRoutes(protected, c.Handlers.Role)

	return r
}

// newTestEngine creates a minimal gin.Engine for testing with TestMode and
// CustomRecovery middleware. It also registers a panic-test route used by
// E2E recovery middleware tests.
func newTestEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.CustomRecovery())

	// Panic test route (only for E2E testing)
	r.GET("/panic-test", func(_ *gin.Context) {
		panic("test panic for recovery middleware")
	})

	return r
}

// registerTestHealthRoutes registers simplified health/ready endpoints for tests.
// Uses health.New() with no checks — always returns healthy (DB connectivity is
// already validated by the test container setup).
func registerTestHealthRoutes(r *gin.Engine, db *sqlx.DB) {
	r.GET("/health", func(c *gin.Context) {
		httpgin.SendSuccess(c, http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/ready", func(c *gin.Context) {
		if pingErr := db.Ping(); pingErr != nil {
			httpgin.SendError(c, http.StatusServiceUnavailable, "database connection failed")
			return
		}
		httpgin.SendSuccess(c, http.StatusOK, gin.H{"status": "ready"})
	})
}
