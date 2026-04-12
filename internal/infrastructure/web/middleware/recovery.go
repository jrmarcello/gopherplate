package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/jrmarcello/gopherplate/pkg/httputil/httpgin"
)

// CustomRecovery returns a middleware that recovers from panics, logs the error
// with a stack trace via slog, and returns a JSON 500 response.
func CustomRecovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		slog.Error("panic recovered",
			"error", recovered,
			"stack", string(debug.Stack()),
		)

		httpgin.SendError(c, http.StatusInternalServerError, "internal server error")
		c.Abort()
	})
}
