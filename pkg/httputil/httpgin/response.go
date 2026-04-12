package httpgin

import (
	"github.com/gin-gonic/gin"
	"github.com/jrmarcello/gopherplate/pkg/httputil"
)

// SendSuccess sends a standardized success response via Gin.
func SendSuccess(c *gin.Context, status int, data any) {
	httputil.WriteSuccess(c.Writer, status, data)
}

// SendSuccessWithMeta sends a standardized success response with metadata and links via Gin.
func SendSuccessWithMeta(c *gin.Context, status int, data, meta, links any) {
	httputil.WriteSuccessWithMeta(c.Writer, status, data, meta, links)
}

// SendError sends a standardized error response via Gin.
func SendError(c *gin.Context, status int, message string) {
	httputil.WriteError(c.Writer, status, message)
}

// SendErrorWithCode sends a standardized error response with an error code via Gin.
func SendErrorWithCode(c *gin.Context, status int, code, message string) {
	httputil.WriteErrorWithCode(c.Writer, status, code, message)
}

// SendErrorWithDetails sends a standardized error response with code and details via Gin.
func SendErrorWithDetails(c *gin.Context, status int, code, message string, details map[string]any) {
	httputil.WriteErrorWithDetails(c.Writer, status, code, message, details)
}
