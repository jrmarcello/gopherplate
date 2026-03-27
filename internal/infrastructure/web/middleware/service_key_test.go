package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"bitbucket.org/appmax-space/go-boilerplate/pkg/logutil"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestParseServiceKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: map[string]string{},
		},
		{
			name:  "single key",
			input: "banking-router:sk_banking_abc123",
			expected: map[string]string{
				"banking-router": "sk_banking_abc123",
			},
		},
		{
			name:  "multiple keys",
			input: "banking-router:sk_banking_abc123,ledger:sk_ledger_xyz789",
			expected: map[string]string{
				"banking-router": "sk_banking_abc123",
				"ledger":         "sk_ledger_xyz789",
			},
		},
		{
			name:  "with whitespace",
			input: " banking-router : sk_banking_abc123 , ledger : sk_ledger_xyz789 ",
			expected: map[string]string{
				"banking-router": "sk_banking_abc123",
				"ledger":         "sk_ledger_xyz789",
			},
		},
		{
			name:     "invalid format - no colon",
			input:    "invalid-format",
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseServiceKeys(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestServiceKeyAuth_NoKeysConfigured(t *testing.T) {
	// Without keys configured (dev mode), all requests should pass
	config := DefaultServiceKeyConfig()

	r := gin.New()
	r.Use(ServiceKeyAuth(config))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestServiceKeyAuth_ValidKey(t *testing.T) {
	config := ServiceKeyConfig{
		Keys: map[string]string{
			"banking-router": "sk_banking_abc123",
		},
	}

	r := gin.New()
	r.Use(ServiceKeyAuth(config))
	r.GET("/test", func(c *gin.Context) {
		lc, _ := logutil.Extract(c.Request.Context())
		c.JSON(http.StatusOK, gin.H{"caller": lc.CallerService})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Service-Name", "banking-router")
	req.Header.Set("X-Service-Key", "sk_banking_abc123")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "banking-router")
}

func TestServiceKeyAuth_InvalidKey(t *testing.T) {
	config := ServiceKeyConfig{
		Keys: map[string]string{
			"banking-router": "sk_banking_abc123",
		},
	}

	r := gin.New()
	r.Use(ServiceKeyAuth(config))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Service-Name", "banking-router")
	req.Header.Set("X-Service-Key", "wrong_key")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "unauthorized")
}

func TestServiceKeyAuth_MissingHeaders(t *testing.T) {
	config := ServiceKeyConfig{
		Keys: map[string]string{
			"banking-router": "sk_banking_abc123",
		},
	}

	r := gin.New()
	r.Use(ServiceKeyAuth(config))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	// No headers set
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "unauthorized")
}

func TestServiceKeyAuth_UnknownService(t *testing.T) {
	config := ServiceKeyConfig{
		Keys: map[string]string{
			"banking-router": "sk_banking_abc123",
		},
	}

	r := gin.New()
	r.Use(ServiceKeyAuth(config))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Service-Name", "unknown-service")
	req.Header.Set("X-Service-Key", "some_key")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "unauthorized")
}
