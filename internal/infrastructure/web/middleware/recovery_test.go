package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jrmarcello/gopherplate/pkg/httputil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestCustomRecovery(t *testing.T) {
	tests := []struct {
		name           string
		handler        gin.HandlerFunc
		expectedStatus int
		expectError    bool
	}{
		{
			name: "TC-U-09: catches string panic and returns JSON 500",
			handler: func(c *gin.Context) {
				panic("something went wrong")
			},
			expectedStatus: http.StatusInternalServerError,
			expectError:    true,
		},
		{
			name: "TC-U-10: passes through when no panic - original handler runs 200 OK",
			handler: func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			},
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name: "TC-U-11: catches error-type panic and returns JSON 500",
			handler: func(c *gin.Context) {
				panic(http.ErrAbortHandler)
			},
			expectedStatus: http.StatusInternalServerError,
			expectError:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.Use(CustomRecovery())
			r.GET("/test", tc.handler)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)

			r.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatus, w.Code)
			assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

			if tc.expectError {
				var resp httputil.ErrorResponse
				decodeErr := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, decodeErr)
				assert.Equal(t, "internal server error", resp.Errors.Message)
			}
		})
	}
}
