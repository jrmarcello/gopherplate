package handler

import (
	"errors"
	"net/http"

	roledomain "bitbucket.org/appmax-space/go-boilerplate/internal/domain/role"
	userdomain "bitbucket.org/appmax-space/go-boilerplate/internal/domain/user"
	"bitbucket.org/appmax-space/go-boilerplate/internal/domain/user/vo"
	"bitbucket.org/appmax-space/go-boilerplate/pkg/apperror"
	"bitbucket.org/appmax-space/go-boilerplate/pkg/httputil/httpgin"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// ErrorResponse represents the standard error response for Swagger documentation.
type ErrorResponse struct {
	Errors struct {
		Message string `json:"message" example:"error description"`
	} `json:"errors"`
}

// codeToStatus maps AppError codes to HTTP status codes.
// This is the single source of truth for error-to-HTTP translation.
var codeToStatus = map[string]int{
	apperror.CodeInvalidRequest:  http.StatusBadRequest,
	apperror.CodeValidationError: http.StatusBadRequest,
	apperror.CodeNotFound:        http.StatusNotFound,
	apperror.CodeConflict:        http.StatusConflict,
	apperror.CodeUnauthorized:    http.StatusUnauthorized,
	apperror.CodeForbidden:       http.StatusForbidden,
	apperror.CodeInternalError:   http.StatusInternalServerError,
}

// HandleError handles errors in a centralized and consistent way.
// It supports AppError (structured) and falls back to domain error translation.
func HandleError(c *gin.Context, span trace.Span, err error) {
	// 1. Try AppError first (structured errors from use cases)
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		status, ok := codeToStatus[appErr.Code]
		if !ok {
			status = http.StatusInternalServerError
		}
		span.SetStatus(codes.Error, appErr.Code)
		if status >= 500 {
			span.RecordError(err)
		}
		httpgin.SendError(c, status, appErr.Message)
		return
	}

	// 2. Fallback: translate domain errors to HTTP
	status, code, message := translateError(err)

	span.SetStatus(codes.Error, code)
	if status >= 500 {
		span.RecordError(err)
	}

	httpgin.SendError(c, status, message)
}

// translateError maps domain errors to HTTP status codes.
// This is the fallback for errors that are not AppError.
func translateError(err error) (status int, code, message string) {
	switch {
	case errors.Is(err, vo.ErrInvalidEmail):
		return http.StatusBadRequest, apperror.CodeInvalidRequest, "invalid email"
	case errors.Is(err, vo.ErrInvalidID):
		return http.StatusBadRequest, apperror.CodeInvalidRequest, "invalid ID"
	case errors.Is(err, userdomain.ErrUserNotFound):
		return http.StatusNotFound, apperror.CodeNotFound, "user not found"
	case errors.Is(err, roledomain.ErrRoleNotFound):
		return http.StatusNotFound, apperror.CodeNotFound, "role not found"
	case errors.Is(err, roledomain.ErrDuplicateRoleName):
		return http.StatusConflict, apperror.CodeConflict, "role name already exists"
	default:
		return http.StatusInternalServerError, apperror.CodeInternalError, "internal server error"
	}
}
