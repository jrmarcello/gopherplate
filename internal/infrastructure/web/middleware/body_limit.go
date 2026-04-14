package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jrmarcello/gopherplate/pkg/httputil/httpgin"
)

// BodyLimit caps incoming request body size to protect against memory-exhaustion
// DoS. It applies two layers of defense:
//
//  1. Early-reject: when the Content-Length header declares a size above the
//     limit, return 413 immediately without reading the body.
//  2. http.MaxBytesReader wrapper: any downstream reader (ShouldBindJSON,
//     the idempotency middleware's io.ReadAll, a handler's io.Copy) cannot
//     consume more than maxBytes, even with chunked transfer encoding or a
//     dishonest Content-Length header.
//
// When maxBytes <= 0 the middleware is a no-op (disabled).
//
// Note on status codes: only the Content-Length early-reject path returns 413.
// When the cap trips inside MaxBytesReader (e.g. chunked encoding), downstream
// handlers translate the bind error into their own response — currently 400
// "invalid request body". The DoS protection is unchanged either way because
// the reader stops at maxBytes regardless of the final status code.
//
// Register this middleware before anything that reads the body (Idempotency,
// route handlers, etc.).
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	if maxBytes <= 0 {
		return func(c *gin.Context) { c.Next() }
	}

	return func(c *gin.Context) {
		if c.Request.ContentLength > maxBytes {
			httpgin.SendError(c, http.StatusRequestEntityTooLarge, "request body too large")
			c.Abort()
			return
		}

		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)

		c.Next()
	}
}
