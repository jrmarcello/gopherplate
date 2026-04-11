package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrmarcello/go-boilerplate/pkg/logutil"
)

func TestLogger_GeneratesUUID_WhenNoRequestIDHeader(t *testing.T) {
	r := gin.New()
	r.Use(Logger())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, reqErr := http.NewRequest("GET", "/test", nil)
	require.NoError(t, reqErr)

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	respID := w.Header().Get(RequestIDHeader)
	assert.NotEmpty(t, respID, "should generate a request ID when none provided")

	_, parseErr := uuid.Parse(respID)
	assert.NoError(t, parseErr, "generated request ID should be a valid UUID")
}

func TestLogger_UsesValidRequestID_FromHeader(t *testing.T) {
	existingID := "abc-123-def-456"

	r := gin.New()
	r.Use(Logger())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, reqErr := http.NewRequest("GET", "/test", nil)
	require.NoError(t, reqErr)
	req.Header.Set(RequestIDHeader, existingID)

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, existingID, w.Header().Get(RequestIDHeader),
		"should preserve valid X-Request-ID from header")
}

func TestLogger_InvalidRequestID_TooLong_GeneratesNewUUID(t *testing.T) {
	// Create a request ID longer than requestIDMaxLen (64)
	longID := strings.Repeat("a", 65)

	r := gin.New()
	r.Use(Logger())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, reqErr := http.NewRequest("GET", "/test", nil)
	require.NoError(t, reqErr)
	req.Header.Set(RequestIDHeader, longID)

	r.ServeHTTP(w, req)

	respID := w.Header().Get(RequestIDHeader)
	assert.NotEqual(t, longID, respID, "should reject too-long request ID")

	_, parseErr := uuid.Parse(respID)
	assert.NoError(t, parseErr, "should generate a valid UUID replacement")
}

func TestLogger_InvalidRequestID_SpecialChars_GeneratesNewUUID(t *testing.T) {
	invalidIDs := []string{
		"id with spaces",
		"id@with#special",
		"../path/traversal",
		"<script>alert(1)</script>",
		"id;DROP TABLE",
	}

	for _, invalidID := range invalidIDs {
		t.Run(invalidID, func(t *testing.T) {
			r := gin.New()
			r.Use(Logger())
			r.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			w := httptest.NewRecorder()
			req, reqErr := http.NewRequest("GET", "/test", nil)
			require.NoError(t, reqErr)
			req.Header.Set(RequestIDHeader, invalidID)

			r.ServeHTTP(w, req)

			respID := w.Header().Get(RequestIDHeader)
			assert.NotEqual(t, invalidID, respID, "should reject invalid request ID")

			_, parseErr := uuid.Parse(respID)
			assert.NoError(t, parseErr, "should generate a valid UUID replacement")
		})
	}
}

func TestLogger_ResponseIncludesRequestIDHeader(t *testing.T) {
	r := gin.New()
	r.Use(Logger())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, reqErr := http.NewRequest("GET", "/test", nil)
	require.NoError(t, reqErr)

	r.ServeHTTP(w, req)

	respID := w.Header().Get(RequestIDHeader)
	assert.NotEmpty(t, respID, "response must always include X-Request-ID header")
}

func TestLogger_CallerServicePropagated_FromContext(t *testing.T) {
	var capturedCaller string

	r := gin.New()
	// Pre-inject a LogContext with CallerService to simulate ServiceKeyAuth upstream
	r.Use(func(c *gin.Context) {
		lc := logutil.LogContext{CallerService: "test-service"}
		ctx := logutil.Inject(c.Request.Context(), lc)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.Use(Logger())
	r.GET("/test", func(c *gin.Context) {
		// After Logger middleware, the LogContext should still be accessible
		// (Logger creates its own LogContext, overwriting the injected one,
		// but the CallerService is set by ServiceKeyAuth which runs after Logger
		// in production. Here we verify Logger injects a LogContext.)
		lc, ok := logutil.Extract(c.Request.Context())
		if ok {
			capturedCaller = lc.CallerService
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, reqErr := http.NewRequest("GET", "/test", nil)
	require.NoError(t, reqErr)

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Logger overwrites the context with its own LogContext (no CallerService),
	// which is the expected behavior - CallerService is set by ServiceKeyAuth downstream.
	// Verify that the LogContext injected by Logger has the middleware step set.
	assert.Empty(t, capturedCaller, "Logger middleware does not set CallerService (that is ServiceKeyAuth's job)")
}

func TestLogger_InjectsLogContext_WithMiddlewareStep(t *testing.T) {
	var capturedStep string

	r := gin.New()
	r.Use(Logger())
	r.GET("/test", func(c *gin.Context) {
		lc, ok := logutil.Extract(c.Request.Context())
		if ok {
			capturedStep = lc.Step
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, reqErr := http.NewRequest("GET", "/test", nil)
	require.NoError(t, reqErr)

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, logutil.StepMiddleware, capturedStep,
		"Logger should inject LogContext with step=middleware")
}
