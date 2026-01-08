package handler

import (
	"errors"
	"net/http"

	"bitbucket.org/appmax-space/ms-boilerplate-go/internal/domain/person"
	"bitbucket.org/appmax-space/ms-boilerplate-go/internal/domain/person/vo"
	"bitbucket.org/appmax-space/ms-boilerplate-go/internal/usecases"
	personuc "bitbucket.org/appmax-space/ms-boilerplate-go/internal/usecases/person"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// ErrorResponse é a estrutura padronizada de resposta de erro.
type ErrorResponse struct {
	Error   string         `json:"error"`
	Code    string         `json:"code"`
	Details map[string]any `json:"details,omitempty"`
	TraceID string         `json:"trace_id,omitempty"`
}

// HandleError trata erros de forma centralizada e consistente.
// Traduz erros de domínio para AppErrors e retorna respostas HTTP apropriadas.
func HandleError(c *gin.Context, span trace.Span, err error) {
	traceID := extractTraceID(span)

	// 1. Tenta converter para AppError primeiro
	var appErr *usecases.AppError
	if errors.As(err, &appErr) {
		respondWithAppError(c, span, appErr, traceID)
		return
	}

	// 2. Traduz erros de domínio para AppError
	if translated := translateDomainError(err); translated != nil {
		respondWithAppError(c, span, translated, traceID)
		return
	}

	// 3. Erro desconhecido - retorna 500
	span.SetStatus(codes.Error, "internal error")
	span.RecordError(err)
	c.JSON(http.StatusInternalServerError, ErrorResponse{
		Error:   "Erro interno do servidor",
		Code:    usecases.CodeInternalError,
		TraceID: traceID,
	})
}

// translateDomainError mapeia erros de domínio puros para AppErrors.
func translateDomainError(err error) *usecases.AppError {
	switch {
	// Erros de Value Object
	case errors.Is(err, vo.ErrInvalidCPF):
		return personuc.ErrInvalidCPF
	case errors.Is(err, vo.ErrInvalidEmail):
		return personuc.ErrInvalidEmail
	case errors.Is(err, vo.ErrInvalidPhone):
		return personuc.ErrInvalidPhone
	// Erros de entidade de domínio
	case errors.Is(err, person.ErrPersonNotFound):
		return personuc.ErrPersonNotFound
	case errors.Is(err, person.ErrDuplicateCPF):
		return personuc.ErrDuplicateCPF
	case errors.Is(err, person.ErrDuplicateEmail):
		return personuc.ErrDuplicateEmail
	default:
		return nil
	}
}

// respondWithAppError envia a resposta de erro e define o status do span.
func respondWithAppError(c *gin.Context, span trace.Span, appErr *usecases.AppError, traceID string) {
	span.SetStatus(codes.Error, appErr.Code)
	if appErr.HTTPStatus >= 500 {
		span.RecordError(appErr)
	}
	c.JSON(appErr.HTTPStatus, ErrorResponse{
		Error:   appErr.Message,
		Code:    appErr.Code,
		Details: appErr.Details,
		TraceID: traceID,
	})
}

// extractTraceID extrai o trace ID do span OpenTelemetry.
func extractTraceID(span trace.Span) string {
	if span.SpanContext().HasTraceID() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}
