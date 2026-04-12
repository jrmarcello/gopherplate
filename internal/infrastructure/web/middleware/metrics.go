package middleware

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jrmarcello/gopherplate/pkg/telemetry"
)

// Metrics returns a middleware that records HTTP request metrics (count, duration, Apdex).
// Skips health check endpoints (/health, /ready) to avoid noise.
func Metrics(httpMetrics *telemetry.HTTPMetrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip health checks
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/health") || strings.HasPrefix(path, "/ready") {
			c.Next()
			return
		}

		start := time.Now()

		c.Next()

		// Record metrics after request completes
		if httpMetrics != nil {
			duration := time.Since(start)
			route := c.FullPath()
			if route == "" {
				route = path // fallback for unmatched routes
			}
			httpMetrics.RecordRequest(
				c.Request.Context(),
				c.Request.Method,
				route,
				c.Writer.Status(),
				duration,
			)
		}
	}
}
