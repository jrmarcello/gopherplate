package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jrmarcello/gopherplate/pkg/httputil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBodyLimit_EarlyRejectOnContentLength(t *testing.T) {
	const maxBytes = int64(1024)

	tests := []struct {
		name           string
		contentLength  int64
		bodySize       int
		expectedStatus int
		expectRead     bool
	}{
		{
			name:           "body within limit is accepted",
			contentLength:  100,
			bodySize:       100,
			expectedStatus: http.StatusOK,
			expectRead:     true,
		},
		{
			name:           "body exactly at limit is accepted",
			contentLength:  maxBytes,
			bodySize:       int(maxBytes),
			expectedStatus: http.StatusOK,
			expectRead:     true,
		},
		{
			name:           "Content-Length above limit triggers early 413 without reading body",
			contentLength:  maxBytes + 1,
			bodySize:       int(maxBytes) + 1,
			expectedStatus: http.StatusRequestEntityTooLarge,
			expectRead:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.Use(BodyLimit(maxBytes))

			var readBytes int64
			r.POST("/test", func(c *gin.Context) {
				n, copyErr := io.Copy(io.Discard, c.Request.Body)
				readBytes = n
				if copyErr != nil {
					c.Status(http.StatusInternalServerError)
					return
				}
				c.Status(http.StatusOK)
			})

			body := bytes.NewReader(bytes.Repeat([]byte("a"), tc.bodySize))
			req := httptest.NewRequest(http.MethodPost, "/test", body)
			req.ContentLength = tc.contentLength
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatus, w.Code)

			if tc.expectedStatus == http.StatusRequestEntityTooLarge {
				var resp httputil.ErrorResponse
				decodeErr := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, decodeErr)
				assert.Equal(t, "request body too large", resp.Errors.Message)
			}

			if !tc.expectRead {
				assert.Equal(t, int64(0), readBytes, "handler must not have read any body after 413")
			}
		})
	}
}

// TestBodyLimit_MaxBytesReaderCapsChunkedBody documents the second layer of
// defense: when Content-Length is absent or lying (e.g. chunked encoding),
// MaxBytesReader still caps the read. Downstream code sees an *http.MaxBytesError
// after maxBytes have been consumed — memory is protected regardless of the
// final HTTP status the handler chooses to return.
func TestBodyLimit_MaxBytesReaderCapsChunkedBody(t *testing.T) {
	const maxBytes = int64(512)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(BodyLimit(maxBytes))

	var readBytes int64
	var readErr error
	r.POST("/test", func(c *gin.Context) {
		n, copyErr := io.Copy(io.Discard, c.Request.Body)
		readBytes = n
		readErr = copyErr
		c.Status(http.StatusOK)
	})

	payloadSize := int(maxBytes) + 1024
	body := bytes.NewReader(bytes.Repeat([]byte("a"), payloadSize))
	req := httptest.NewRequest(http.MethodPost, "/test", body)
	req.ContentLength = -1
	req.TransferEncoding = []string{"chunked"}
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Error(t, readErr, "read must fail once cap is reached")
	var maxBytesErr *http.MaxBytesError
	assert.True(t, errors.As(readErr, &maxBytesErr), "error must be *http.MaxBytesError")
	assert.LessOrEqual(t, readBytes, maxBytes, "handler must not read past the cap")
}

func TestBodyLimit_ZeroDisables(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(BodyLimit(0))

	payload := bytes.Repeat([]byte("a"), 10_000)
	r.POST("/test", func(c *gin.Context) {
		n, copyErr := io.Copy(io.Discard, c.Request.Body)
		require.NoError(t, copyErr)
		assert.Equal(t, int64(len(payload)), n, "no cap should be applied when maxBytes <= 0")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(payload))
	req.ContentLength = int64(len(payload))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestBodyLimit_AllowsNormalJSONPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(BodyLimit(1 << 20))
	r.POST("/echo", func(c *gin.Context) {
		payload, readErr := io.ReadAll(c.Request.Body)
		require.NoError(t, readErr)
		c.String(http.StatusOK, string(payload))
	})

	body := strings.NewReader(`{"name":"alice","email":"alice@example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/echo", body)
	req.ContentLength = int64(body.Len())
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, `{"name":"alice","email":"alice@example.com"}`, w.Body.String())
}
