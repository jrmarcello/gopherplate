package handler

import (
	"errors"
	"net/http"

	"bitbucket.org/appmax-space/go-boilerplate/internal/domain/entity"
	"bitbucket.org/appmax-space/go-boilerplate/internal/domain/entity/vo"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Error codes
const (
	CodeInternalError = "INTERNAL_ERROR"
	CodeNotFound      = "NOT_FOUND"
	CodeInvalidEmail  = "INVALID_EMAIL"
	CodeBadRequest    = "BAD_REQUEST"
)

// ErrorResponse é a estrutura padronizada de resposta de erro.
type ErrorResponse struct {
	Error   string         `json:"error"`
	Code    string         `json:"code"`
	Details map[string]any `json:"details,omitempty"`
	TraceID string         `json:"trace_id,omitempty"`
}

// HandleError trata erros de forma centralizada e consistente.
// Traduz erros de domínio para respostas HTTP apropriadas.
func HandleError(c *gin.Context, span trace.Span, err error) {
	traceID := extractTraceID(span)

	// Traduz erros de domínio
	status, code, message := translateError(err)

	span.SetStatus(codes.Error, code)
	if status >= 500 {
		span.RecordError(err)
	}

	c.JSON(status, ErrorResponse{
		Error:   message,
		Code:    code,
		TraceID: traceID,
	})
}

// translateError mapeia erros de domínio para códigos HTTP.
func translateError(err error) (status int, code, message string) {
	switch {
	case errors.Is(err, vo.ErrInvalidEmail):
		return http.StatusBadRequest, CodeInvalidEmail, "Email inválido"
	case errors.Is(err, entity.ErrEntityNotFound):
		return http.StatusNotFound, CodeNotFound, "Entity não encontrada"
	default:
		// Erro com mensagem "invalid ULID" do vo.ParseID
		if err != nil && err.Error() == "invalid ULID" {
			return http.StatusBadRequest, CodeBadRequest, "ID inválido"
		}
		return http.StatusInternalServerError, CodeInternalError, "Erro interno do servidor"
	}
}

// extractTraceID extrai o trace ID do span OpenTelemetry.
func extractTraceID(span trace.Span) string {
	if span.SpanContext().HasTraceID() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}
