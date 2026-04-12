package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jrmarcello/gopherplate/pkg/apperror"
	"github.com/jrmarcello/gopherplate/pkg/httputil/httpgin"
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
	apperror.CodeInvalidRequest:      http.StatusBadRequest,
	apperror.CodeValidationError:     http.StatusBadRequest,
	apperror.CodeNotFound:            http.StatusNotFound,
	apperror.CodeConflict:            http.StatusConflict,
	apperror.CodeUnauthorized:        http.StatusUnauthorized,
	apperror.CodeForbidden:           http.StatusForbidden,
	apperror.CodeUnprocessableEntity: http.StatusUnprocessableEntity,
	apperror.CodeInternalError:       http.StatusInternalServerError,
}

// HandleError handles errors in a centralized and consistent way.
// It maps AppError codes to HTTP status codes via codeToStatus.
// Non-AppError errors return a generic 500 Internal Server Error.
func HandleError(c *gin.Context, err error) {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		status, ok := codeToStatus[appErr.Code]
		if !ok {
			status = http.StatusInternalServerError
		}
		httpgin.SendError(c, status, appErr.Message)
		return
	}

	httpgin.SendError(c, http.StatusInternalServerError, "internal server error")
}
