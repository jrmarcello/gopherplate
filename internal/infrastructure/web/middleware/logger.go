package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

const (
	// RequestIDHeader é o header usado para o Request ID
	RequestIDHeader = "X-Request-ID"
)

// Logger retorna um middleware de logging estruturado
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Gerar ou usar Request ID existente
		requestID := c.GetHeader(RequestIDHeader)
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set("request_id", requestID)
		c.Header(RequestIDHeader, requestID)

		// Extrair Trace ID se disponível
		traceID := ""
		span := trace.SpanFromContext(c.Request.Context())
		if span.SpanContext().HasTraceID() {
			traceID = span.SpanContext().TraceID().String()
		}

		// Log de entrada
		slog.Info("request started",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"request_id", requestID,
			"trace_id", traceID,
			"client_ip", c.ClientIP(),
		)

		// Processar request
		c.Next()

		// Log de saída
		duration := time.Since(start)
		status := c.Writer.Status()

		logLevel := slog.LevelInfo
		if status >= 500 {
			logLevel = slog.LevelError
		} else if status >= 400 {
			logLevel = slog.LevelWarn
		}

		slog.Log(c.Request.Context(), logLevel, "request completed",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", status,
			"duration_ms", duration.Milliseconds(),
			"request_id", requestID,
			"trace_id", traceID,
		)
	}
}
