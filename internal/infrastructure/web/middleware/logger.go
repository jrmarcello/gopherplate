package middleware

import (
	"log/slog"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"

	"bitbucket.org/appmax-space/go-boilerplate/pkg/logutil"
)

const requestIDMaxLen = 64

// validRequestID matches strings containing only alphanumeric characters and hyphens.
var validRequestID = regexp.MustCompile(`^[a-zA-Z0-9-]+$`)

const (
	// RequestIDHeader é o header usado para o Request ID
	RequestIDHeader = "X-Request-ID"
)

// Logger retorna um middleware de logging estruturado
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Gerar ou usar Request ID existente (sanitized)
		requestID := c.GetHeader(RequestIDHeader)
		if requestID == "" || len(requestID) > requestIDMaxLen || !validRequestID.MatchString(requestID) {
			requestID = uuid.New().String()
		}
		c.Header(RequestIDHeader, requestID)

		// Extrair Trace ID se disponível
		traceID := ""
		span := trace.SpanFromContext(c.Request.Context())
		if span.SpanContext().HasTraceID() {
			traceID = span.SpanContext().TraceID().String()
		}

		// Inject LogContext into request context for downstream use
		lc := logutil.LogContext{
			RequestID: requestID,
			TraceID:   traceID,
			Step:      logutil.StepMiddleware,
		}
		ctx := logutil.Inject(c.Request.Context(), lc)
		c.Request = c.Request.WithContext(ctx)

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
