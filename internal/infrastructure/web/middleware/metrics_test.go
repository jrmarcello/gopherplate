package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrmarcello/go-boilerplate/pkg/telemetry"
)

func TestMetrics_NilMetrics_GracefulNoOp(t *testing.T) {
	r := gin.New()
	r.Use(Metrics(nil))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, reqErr := http.NewRequest("GET", "/test", nil)
	require.NoError(t, reqErr)

	// Should not panic with nil metrics
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMetrics_RecordsOnSuccessfulRequest(t *testing.T) {
	httpMetrics, metricsErr := telemetry.NewHTTPMetrics("test-service")
	require.NoError(t, metricsErr)

	r := gin.New()
	r.Use(Metrics(httpMetrics))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, reqErr := http.NewRequest("GET", "/test", nil)
	require.NoError(t, reqErr)

	// Should not panic and should complete successfully
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMetrics_RecordsOnErrorRequest(t *testing.T) {
	httpMetrics, metricsErr := telemetry.NewHTTPMetrics("test-service")
	require.NoError(t, metricsErr)

	r := gin.New()
	r.Use(Metrics(httpMetrics))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "something went wrong"})
	})

	w := httptest.NewRecorder()
	req, reqErr := http.NewRequest("GET", "/test", nil)
	require.NoError(t, reqErr)

	// Should not panic and should complete successfully even on error status
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestMetrics_SkipsHealthCheckEndpoints(t *testing.T) {
	httpMetrics, metricsErr := telemetry.NewHTTPMetrics("test-service")
	require.NoError(t, metricsErr)

	r := gin.New()
	r.Use(Metrics(httpMetrics))
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Test /health
	w1 := httptest.NewRecorder()
	req1, reqErr1 := http.NewRequest("GET", "/health", nil)
	require.NoError(t, reqErr1)
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	// Test /ready
	w2 := httptest.NewRecorder()
	req2, reqErr2 := http.NewRequest("GET", "/ready", nil)
	require.NoError(t, reqErr2)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestMetrics_FallbackRoute_WhenNoMatchedRoute(t *testing.T) {
	httpMetrics, metricsErr := telemetry.NewHTTPMetrics("test-service")
	require.NoError(t, metricsErr)

	r := gin.New()
	r.Use(Metrics(httpMetrics))
	// NoRoute handler for unmatched paths
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	})

	w := httptest.NewRecorder()
	req, reqErr := http.NewRequest("GET", "/unknown-path", nil)
	require.NoError(t, reqErr)

	// Should not panic even when FullPath() returns empty
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
