package httpgin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jrmarcello/go-boilerplate/pkg/httputil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRouter(handler gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/test", handler)
	return r
}

func performRequest(t *testing.T, r *gin.Engine) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req, reqErr := http.NewRequest("GET", "/test", nil)
	require.NoError(t, reqErr)
	r.ServeHTTP(w, req)
	return w
}

func TestSendSuccess(t *testing.T) {
	t.Run("returns data with status 200", func(t *testing.T) {
		r := setupTestRouter(func(c *gin.Context) {
			SendSuccess(c, http.StatusOK, gin.H{"id": "123", "name": "Test"})
		})

		w := performRequest(t, r)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp httputil.SuccessResponse
		parseErr := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, parseErr)
		assert.NotNil(t, resp.Data)
	})

	t.Run("returns nil data", func(t *testing.T) {
		r := setupTestRouter(func(c *gin.Context) {
			SendSuccess(c, http.StatusOK, nil)
		})

		w := performRequest(t, r)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		parseErr := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, parseErr)
		assert.Nil(t, resp["data"])
	})

	t.Run("returns status 201 Created", func(t *testing.T) {
		r := setupTestRouter(func(c *gin.Context) {
			SendSuccess(c, http.StatusCreated, gin.H{"id": "456"})
		})

		w := performRequest(t, r)

		assert.Equal(t, http.StatusCreated, w.Code)

		var resp httputil.SuccessResponse
		parseErr := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, parseErr)
		assert.NotNil(t, resp.Data)
	})

	t.Run("returns status 204 No Content with nil data", func(t *testing.T) {
		r := setupTestRouter(func(c *gin.Context) {
			SendSuccess(c, http.StatusNoContent, nil)
		})

		w := performRequest(t, r)

		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("response Content-Type is application/json", func(t *testing.T) {
		r := setupTestRouter(func(c *gin.Context) {
			SendSuccess(c, http.StatusOK, gin.H{"ok": true})
		})

		w := performRequest(t, r)

		assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	})

	t.Run("meta and links are omitted when not provided", func(t *testing.T) {
		r := setupTestRouter(func(c *gin.Context) {
			SendSuccess(c, http.StatusOK, gin.H{"id": "1"})
		})

		w := performRequest(t, r)

		var raw map[string]any
		parseErr := json.Unmarshal(w.Body.Bytes(), &raw)
		require.NoError(t, parseErr)
		_, hasMeta := raw["meta"]
		_, hasLinks := raw["links"]
		assert.False(t, hasMeta, "meta should be omitted")
		assert.False(t, hasLinks, "links should be omitted")
	})
}

func TestSendError(t *testing.T) {
	statusCases := []struct {
		name    string
		status  int
		message string
	}{
		{"400 Bad Request", http.StatusBadRequest, "invalid request"},
		{"401 Unauthorized", http.StatusUnauthorized, "authentication required"},
		{"403 Forbidden", http.StatusForbidden, "access denied"},
		{"404 Not Found", http.StatusNotFound, "resource not found"},
		{"500 Internal Server Error", http.StatusInternalServerError, "internal error"},
	}

	for _, tc := range statusCases {
		t.Run(tc.name, func(t *testing.T) {
			r := setupTestRouter(func(c *gin.Context) {
				SendError(c, tc.status, tc.message)
			})

			w := performRequest(t, r)

			assert.Equal(t, tc.status, w.Code)
			assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

			var resp httputil.ErrorResponse
			parseErr := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, parseErr)
			assert.Equal(t, tc.message, resp.Errors.Message)
		})
	}
}

func TestSendSuccessWithMeta(t *testing.T) {
	t.Run("with meta and links populated", func(t *testing.T) {
		r := setupTestRouter(func(c *gin.Context) {
			data := []string{"a", "b"}
			meta := gin.H{"total": 2, "page": 1}
			links := gin.H{"next": "/test?page=2", "prev": "/test?page=0"}
			SendSuccessWithMeta(c, http.StatusOK, data, meta, links)
		})

		w := performRequest(t, r)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		parseErr := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, parseErr)
		assert.NotNil(t, resp["data"])
		assert.NotNil(t, resp["meta"])
		assert.NotNil(t, resp["links"])

		metaMap, ok := resp["meta"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, float64(2), metaMap["total"])
		assert.Equal(t, float64(1), metaMap["page"])

		linksMap, ok := resp["links"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "/test?page=2", linksMap["next"])
	})

	t.Run("with nil meta", func(t *testing.T) {
		r := setupTestRouter(func(c *gin.Context) {
			SendSuccessWithMeta(c, http.StatusOK, []string{"x"}, nil, gin.H{"self": "/test"})
		})

		w := performRequest(t, r)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		parseErr := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, parseErr)
		_, hasMeta := resp["meta"]
		assert.False(t, hasMeta, "nil meta should be omitted due to omitempty")
		assert.NotNil(t, resp["links"])
	})

	t.Run("with nil links", func(t *testing.T) {
		r := setupTestRouter(func(c *gin.Context) {
			SendSuccessWithMeta(c, http.StatusOK, []string{"x"}, gin.H{"total": 1}, nil)
		})

		w := performRequest(t, r)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		parseErr := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, parseErr)
		assert.NotNil(t, resp["meta"])
		_, hasLinks := resp["links"]
		assert.False(t, hasLinks, "nil links should be omitted due to omitempty")
	})

	t.Run("with both meta and links nil", func(t *testing.T) {
		r := setupTestRouter(func(c *gin.Context) {
			SendSuccessWithMeta(c, http.StatusOK, gin.H{"id": "1"}, nil, nil)
		})

		w := performRequest(t, r)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		parseErr := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, parseErr)
		assert.NotNil(t, resp["data"])
		_, hasMeta := resp["meta"]
		_, hasLinks := resp["links"]
		assert.False(t, hasMeta, "nil meta should be omitted")
		assert.False(t, hasLinks, "nil links should be omitted")
	})

	t.Run("response Content-Type is application/json", func(t *testing.T) {
		r := setupTestRouter(func(c *gin.Context) {
			SendSuccessWithMeta(c, http.StatusOK, "data", gin.H{}, gin.H{})
		})

		w := performRequest(t, r)

		assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	})
}

func TestSendErrorWithCode(t *testing.T) {
	t.Run("includes error code in response", func(t *testing.T) {
		r := setupTestRouter(func(c *gin.Context) {
			SendErrorWithCode(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "field is required")
		})

		w := performRequest(t, r)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

		var resp httputil.ErrorResponse
		parseErr := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, parseErr)
		assert.Equal(t, "VALIDATION_ERROR", resp.Errors.Code)
		assert.Equal(t, "field is required", resp.Errors.Message)
	})

	t.Run("response Content-Type is application/json", func(t *testing.T) {
		r := setupTestRouter(func(c *gin.Context) {
			SendErrorWithCode(c, http.StatusBadRequest, "BAD_REQUEST", "bad")
		})

		w := performRequest(t, r)

		assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	})
}

func TestSendErrorWithDetails(t *testing.T) {
	t.Run("includes code and details in response", func(t *testing.T) {
		details := map[string]any{
			"field":  "email",
			"reason": "invalid format",
		}
		r := setupTestRouter(func(c *gin.Context) {
			SendErrorWithDetails(c, http.StatusBadRequest, "VALIDATION_ERROR", "validation failed", details)
		})

		w := performRequest(t, r)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var resp httputil.ErrorResponse
		parseErr := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, parseErr)
		assert.Equal(t, "VALIDATION_ERROR", resp.Errors.Code)
		assert.Equal(t, "validation failed", resp.Errors.Message)
		assert.Equal(t, "email", resp.Errors.Details["field"])
		assert.Equal(t, "invalid format", resp.Errors.Details["reason"])
	})

	t.Run("with nil details", func(t *testing.T) {
		r := setupTestRouter(func(c *gin.Context) {
			SendErrorWithDetails(c, http.StatusBadRequest, "ERR", "error msg", nil)
		})

		w := performRequest(t, r)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var raw map[string]any
		parseErr := json.Unmarshal(w.Body.Bytes(), &raw)
		require.NoError(t, parseErr)

		errorsMap, ok := raw["errors"].(map[string]any)
		require.True(t, ok)
		_, hasDetails := errorsMap["details"]
		assert.False(t, hasDetails, "nil details should be omitted due to omitempty")
	})
}
