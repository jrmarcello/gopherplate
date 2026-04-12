package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jrmarcello/go-boilerplate/pkg/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// errorBody is the JSON structure returned by SendError.
type errorBody struct {
	Errors struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func TestHandleError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantStatus  int
		wantMessage string
	}{
		{
			name:        "TC-U-12: maps INVALID_REQUEST to 400",
			err:         apperror.New(apperror.CodeInvalidRequest, "invalid email format"),
			wantStatus:  http.StatusBadRequest,
			wantMessage: "invalid email format",
		},
		{
			name:        "TC-U-13: maps NOT_FOUND to 404",
			err:         apperror.New(apperror.CodeNotFound, "user not found"),
			wantStatus:  http.StatusNotFound,
			wantMessage: "user not found",
		},
		{
			name:        "TC-U-14: maps CONFLICT to 409",
			err:         apperror.New(apperror.CodeConflict, "email already exists"),
			wantStatus:  http.StatusConflict,
			wantMessage: "email already exists",
		},
		{
			name:        "TC-U-15: maps UNPROCESSABLE_ENTITY to 422",
			err:         apperror.New(apperror.CodeUnprocessableEntity, "cannot delete active user"),
			wantStatus:  http.StatusUnprocessableEntity,
			wantMessage: "cannot delete active user",
		},
		{
			name:        "TC-U-16: non-AppError returns 500",
			err:         errors.New("unexpected database timeout"),
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			HandleError(c, tt.err)

			assert.Equal(t, tt.wantStatus, w.Code)

			var body errorBody
			decodeErr := json.NewDecoder(w.Body).Decode(&body)
			require.NoError(t, decodeErr)
			assert.Equal(t, tt.wantMessage, body.Errors.Message)
		})
	}
}

func TestHandleError_UnknownAppErrorCode(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	unknownErr := apperror.New("UNKNOWN_CODE", "something weird happened")
	HandleError(c, unknownErr)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var body errorBody
	decodeErr := json.NewDecoder(w.Body).Decode(&body)
	require.NoError(t, decodeErr)
	assert.Equal(t, "something weird happened", body.Errors.Message)
}
