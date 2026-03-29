package apperror

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppError_WithDetails(t *testing.T) {
	original := BadRequest(CodeValidationError, "validation failed")
	details := map[string]any{"field": "email", "reason": "invalid format"}

	withDetails := original.WithDetails(details)

	assert.Equal(t, details, withDetails.Details)
	assert.Nil(t, original.Details) // original is not mutated
}

func TestAppError_WithError(t *testing.T) {
	original := Internal(CodeInternalError, "something went wrong")
	cause := errors.New("db connection failed")

	withErr := original.WithError(cause)

	assert.Equal(t, cause, withErr.Err)
	assert.Nil(t, original.Err) // original is not mutated
	assert.Contains(t, withErr.Error(), "db connection failed")
}

func TestConstructors(t *testing.T) {
	tests := []struct {
		name        string
		constructor func(string, string) *AppError
	}{
		{"BadRequest", BadRequest},
		{"NotFound", NotFound},
		{"Conflict", Conflict},
		{"Internal", Internal},
		{"Unauthorized", Unauthorized},
		{"Forbidden", Forbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appErr := tt.constructor("CODE", "message")
			assert.Equal(t, "CODE", appErr.Code)
			assert.Equal(t, "message", appErr.Message)
		})
	}
}

func TestWrap(t *testing.T) {
	cause := errors.New("original error")
	wrapped := Wrap(cause, CodeInternalError, "wrapped message")

	assert.Equal(t, cause, wrapped.Err)
	assert.Equal(t, CodeInternalError, wrapped.Code)
	assert.Contains(t, wrapped.Error(), "original error")
}

func TestErrorsAs(t *testing.T) {
	appErr := NotFound(CodeNotFound, "user not found")
	var target *AppError
	assert.True(t, errors.As(appErr, &target))
	assert.Equal(t, CodeNotFound, target.Code)
}

func TestAppError_Error_WithoutCause(t *testing.T) {
	appErr := BadRequest(CodeInvalidRequest, "invalid email")

	assert.Equal(t, "invalid email", appErr.Error())
}

func TestAppError_Error_WithCause(t *testing.T) {
	cause := errors.New("db timeout")
	appErr := Internal(CodeInternalError, "operation failed").WithError(cause)

	assert.Equal(t, "operation failed: db timeout", appErr.Error())
}

func TestUnwrap(t *testing.T) {
	t.Run("with cause returns the wrapped error", func(t *testing.T) {
		cause := errors.New("db connection failed")
		appErr := Internal(CodeInternalError, "something went wrong").WithError(cause)

		assert.Equal(t, cause, appErr.Unwrap())
	})

	t.Run("without cause returns nil", func(t *testing.T) {
		appErr := Internal(CodeInternalError, "something went wrong")

		assert.Nil(t, appErr.Unwrap())
	})

	t.Run("errors.Is works through the chain", func(t *testing.T) {
		sentinel := errors.New("sentinel error")
		appErr := Wrap(sentinel, CodeInternalError, "wrapped")

		assert.True(t, errors.Is(appErr, sentinel))
	})

	t.Run("errors.Unwrap stdlib compatibility", func(t *testing.T) {
		cause := errors.New("root cause")
		appErr := Internal(CodeInternalError, "top level").WithError(cause)

		unwrapped := errors.Unwrap(appErr)
		assert.Equal(t, cause, unwrapped)
	})
}
