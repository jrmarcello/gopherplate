package apperror

import "fmt"

// Common error codes
const (
	CodeInternalError   = "INTERNAL_ERROR"
	CodeInvalidRequest  = "INVALID_REQUEST"
	CodeValidationError = "VALIDATION_ERROR"
	CodeNotFound        = "NOT_FOUND"
	CodeConflict        = "CONFLICT"
	CodeUnauthorized    = "UNAUTHORIZED"
	CodeForbidden       = "FORBIDDEN"
)

// AppError is the base application error.
// It implements the error interface and supports unwrapping.
type AppError struct {
	Code    string         // Unique code (e.g., "INVALID_EMAIL")
	Message string         // User-friendly message
	Details map[string]any // Extra details (field, invalid value, etc.)
	Err     error          // Original error (for wrapping)
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap allows using errors.Is() and errors.As() with the original error.
func (e *AppError) Unwrap() error {
	return e.Err
}

// WithDetails returns a copy with additional details.
func (e *AppError) WithDetails(details map[string]any) *AppError {
	newErr := *e
	newErr.Details = details
	return &newErr
}

// WithError returns a copy with the original error wrapped.
func (e *AppError) WithError(err error) *AppError {
	newErr := *e
	newErr.Err = err
	return &newErr
}

// =============================================================================
// Constructors
// =============================================================================

// New creates a new AppError.
func New(code, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// BadRequest creates a bad request error.
func BadRequest(code, message string) *AppError {
	return New(code, message)
}

// NotFound creates a not found error.
func NotFound(code, message string) *AppError {
	return New(code, message)
}

// Conflict creates a conflict error.
func Conflict(code, message string) *AppError {
	return New(code, message)
}

// Internal creates an internal server error.
func Internal(code, message string) *AppError {
	return New(code, message)
}

// Unauthorized creates an unauthorized error.
func Unauthorized(code, message string) *AppError {
	return New(code, message)
}

// Forbidden creates a forbidden error.
func Forbidden(code, message string) *AppError {
	return New(code, message)
}

// Wrap wraps an existing error into an AppError.
func Wrap(err error, code, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}
