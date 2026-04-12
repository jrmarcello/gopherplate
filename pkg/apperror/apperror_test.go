package apperror

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TC-U-18: CodeUnprocessableEntity constant and UnprocessableEntity constructor
func TestUnprocessableEntity(t *testing.T) {
	tests := []struct {
		name            string
		message         string
		expectedCode    string
		expectedMessage string
	}{
		{
			name:            "creates error with correct code and message",
			message:         "invalid input data",
			expectedCode:    "UNPROCESSABLE_ENTITY",
			expectedMessage: "invalid input data",
		},
		{
			name:            "creates error with empty message",
			message:         "",
			expectedCode:    "UNPROCESSABLE_ENTITY",
			expectedMessage: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appErr := UnprocessableEntity(tt.message)

			require.NotNil(t, appErr)
			assert.Equal(t, tt.expectedCode, appErr.Code)
			assert.Equal(t, tt.expectedMessage, appErr.Message)
			assert.Nil(t, appErr.Err)
			assert.Nil(t, appErr.Details)
		})
	}
}

func TestCodeUnprocessableEntityConstant(t *testing.T) {
	assert.Equal(t, "UNPROCESSABLE_ENTITY", CodeUnprocessableEntity)
}

// TC-U-17: Wrap preserves original error chain — errors.Is(wrapped, original) == true
func TestWrapPreservesErrorChain(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() (wrapped *AppError, original error)
		checkIs bool
	}{
		{
			name: "wrapped error is detectable via errors.Is",
			setup: func() (wrapped *AppError, original error) {
				originalErr := errors.New("database connection failed")
				wrappedErr := Wrap(originalErr, CodeInternalError, "failed to create user")
				return wrappedErr, originalErr
			},
			checkIs: true,
		},
		{
			name: "deeply nested error chain is preserved",
			setup: func() (wrapped *AppError, original error) {
				midErr := errors.New("query failed")
				chainedErr := Wrap(midErr, CodeInternalError, "repository error")
				outerErr := Wrap(chainedErr, CodeInternalError, "use case error")
				return outerErr, chainedErr
			},
			checkIs: true,
		},
		{
			name: "Unwrap returns the original error",
			setup: func() (wrapped *AppError, original error) {
				originalErr := errors.New("original")
				wrappedErr := Wrap(originalErr, CodeNotFound, "not found")
				return wrappedErr, originalErr
			},
			checkIs: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrappedErr, originalErr := tt.setup()

			if tt.checkIs {
				assert.True(t, errors.Is(wrappedErr, originalErr),
					"errors.Is should find original error in chain")
			}

			assert.NotNil(t, wrappedErr.Unwrap(), "Unwrap should return the wrapped error")
		})
	}
}

func TestWrapUnwrapReturnsInnerError(t *testing.T) {
	innerErr := errors.New("inner error")
	wrappedErr := Wrap(innerErr, CodeInternalError, "outer")

	assert.Equal(t, innerErr, wrappedErr.Unwrap())
}

func TestWrapErrorMessage(t *testing.T) {
	innerErr := errors.New("connection refused")
	wrappedErr := Wrap(innerErr, CodeInternalError, "database error")

	assert.Equal(t, "database error: connection refused", wrappedErr.Error())
}

func TestWrapWithErrorsAs(t *testing.T) {
	originalAppErr := BadRequest(CodeInvalidRequest, "bad input")
	wrappedErr := Wrap(originalAppErr, CodeInternalError, "processing failed")

	var targetErr *AppError
	assert.True(t, errors.As(wrappedErr, &targetErr),
		"errors.As should find AppError in chain")
	assert.Equal(t, CodeInternalError, targetErr.Code)

	// The inner AppError should also be findable
	innerUnwrapped := wrappedErr.Unwrap()
	var innerAppErr *AppError
	assert.True(t, errors.As(innerUnwrapped, &innerAppErr),
		"inner error should also be an AppError")
	assert.Equal(t, CodeInvalidRequest, innerAppErr.Code)
}
