package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrmarcello/gopherplate/pkg/httputil"
	"github.com/jrmarcello/gopherplate/pkg/idempotency"
)

// --- Mock Store ---

// mockStore implements idempotency.Store for testing.
type mockStore struct {
	mu      sync.Mutex
	entries map[string]*idempotency.Entry

	// Control behavior for testing
	lockErr     error
	getErr      error
	completeErr error
	unlockErr   error

	// Track calls
	lockCalls     int
	completeCalls int
	unlockCalls   int
}

func newMockStore() *mockStore {
	return &mockStore{
		entries: make(map[string]*idempotency.Entry),
	}
}

func (m *mockStore) Lock(_ context.Context, key string, fingerprint string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lockCalls++

	if m.lockErr != nil {
		return false, m.lockErr
	}

	if _, exists := m.entries[key]; exists {
		return false, nil
	}

	m.entries[key] = &idempotency.Entry{
		Status:      idempotency.StatusProcessing,
		Fingerprint: fingerprint,
	}
	return true, nil
}

func (m *mockStore) Get(_ context.Context, key string) (*idempotency.Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.getErr != nil {
		return nil, m.getErr
	}

	entry, exists := m.entries[key]
	if !exists {
		return nil, nil
	}
	return entry, nil
}

func (m *mockStore) Complete(_ context.Context, key string, entry *idempotency.Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completeCalls++

	if m.completeErr != nil {
		return m.completeErr
	}

	entry.Status = idempotency.StatusCompleted
	m.entries[key] = entry
	return nil
}

func (m *mockStore) Unlock(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.unlockCalls++

	if m.unlockErr != nil {
		return m.unlockErr
	}

	delete(m.entries, key)
	return nil
}

// --- Tests ---

func TestIdempotency_NonPOSTRequest_PassesThrough(t *testing.T) {
	store := newMockStore()

	r := gin.New()
	r.Use(Idempotency(store))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, reqErr := http.NewRequest("GET", "/test", nil)
	require.NoError(t, reqErr)
	req.Header.Set(IdempotencyKeyHeader, "some-key")

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 0, store.lockCalls, "should not call Lock for non-POST requests")
}

func TestIdempotency_POSTWithoutKey_PassesThrough(t *testing.T) {
	store := newMockStore()

	r := gin.New()
	r.Use(Idempotency(store))
	r.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"id": "123"})
	})

	body := []byte(`{"name":"test"}`)
	w := httptest.NewRecorder()
	req, reqErr := http.NewRequest("POST", "/test", bytes.NewBuffer(body))
	require.NoError(t, reqErr)
	// No Idempotency-Key header

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, 0, store.lockCalls, "should not call Lock without Idempotency-Key header")
}

func TestIdempotency_FirstPOSTWithKey_AcquiresLockAndProceeds(t *testing.T) {
	store := newMockStore()

	handlerCalled := false
	r := gin.New()
	r.Use(Idempotency(store))
	r.POST("/test", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(http.StatusCreated, gin.H{"id": "123"})
	})

	body := []byte(`{"name":"test"}`)
	w := httptest.NewRecorder()
	req, reqErr := http.NewRequest("POST", "/test", bytes.NewBuffer(body))
	require.NoError(t, reqErr)
	req.Header.Set(IdempotencyKeyHeader, "unique-key-1")

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.True(t, handlerCalled, "handler should be called on first request")
	assert.Equal(t, 1, store.lockCalls, "should call Lock once")
	assert.Equal(t, 1, store.completeCalls, "should call Complete for 2xx response")
}

func TestIdempotency_DuplicatePOST_SameBody_ReplaysResponse(t *testing.T) {
	store := newMockStore()

	handlerCallCount := 0
	r := gin.New()
	r.Use(Idempotency(store))
	r.POST("/test", func(c *gin.Context) {
		handlerCallCount++
		c.JSON(http.StatusCreated, gin.H{"id": "123"})
	})

	body := []byte(`{"name":"test"}`)

	// First request
	w1 := httptest.NewRecorder()
	req1, reqErr1 := http.NewRequest("POST", "/test", bytes.NewBuffer(body))
	require.NoError(t, reqErr1)
	req1.Header.Set(IdempotencyKeyHeader, "dup-key-1")
	r.ServeHTTP(w1, req1)

	assert.Equal(t, http.StatusCreated, w1.Code)
	assert.Equal(t, 1, handlerCallCount)

	// Second request with same key and same body
	w2 := httptest.NewRecorder()
	req2, reqErr2 := http.NewRequest("POST", "/test", bytes.NewBuffer(body))
	require.NoError(t, reqErr2)
	req2.Header.Set(IdempotencyKeyHeader, "dup-key-1")
	r.ServeHTTP(w2, req2)

	// Should replay the stored response
	assert.Equal(t, http.StatusCreated, w2.Code)
	assert.Equal(t, 1, handlerCallCount, "handler should NOT be called again for duplicate request")
}

func TestIdempotency_DuplicatePOST_DifferentBody_Returns422(t *testing.T) {
	store := newMockStore()

	r := gin.New()
	r.Use(Idempotency(store))
	r.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"id": "123"})
	})

	// First request
	body1 := []byte(`{"name":"first"}`)
	w1 := httptest.NewRecorder()
	req1, reqErr1 := http.NewRequest("POST", "/test", bytes.NewBuffer(body1))
	require.NoError(t, reqErr1)
	req1.Header.Set(IdempotencyKeyHeader, "dup-key-2")
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusCreated, w1.Code)

	// Second request with same key but different body
	body2 := []byte(`{"name":"different"}`)
	w2 := httptest.NewRecorder()
	req2, reqErr2 := http.NewRequest("POST", "/test", bytes.NewBuffer(body2))
	require.NoError(t, reqErr2)
	req2.Header.Set(IdempotencyKeyHeader, "dup-key-2")
	r.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusUnprocessableEntity, w2.Code)

	var errResp httputil.ErrorResponse
	unmarshalErr := json.Unmarshal(w2.Body.Bytes(), &errResp)
	require.NoError(t, unmarshalErr)
	assert.Contains(t, errResp.Errors.Message, "different request body")
}

func TestIdempotency_ConcurrentPOST_WhileProcessing_Returns409(t *testing.T) {
	store := newMockStore()

	// Pre-populate the store with a PROCESSING entry to simulate a concurrent request
	fp := bodyFingerprint([]byte(`{"name":"test"}`))
	store.mu.Lock()
	store.entries["idempotency:concurrent-key"] = &idempotency.Entry{
		Status:      idempotency.StatusProcessing,
		Fingerprint: fp,
	}
	store.mu.Unlock()

	r := gin.New()
	r.Use(Idempotency(store))
	r.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"id": "123"})
	})

	body := []byte(`{"name":"test"}`)
	w := httptest.NewRecorder()
	req, reqErr := http.NewRequest("POST", "/test", bytes.NewBuffer(body))
	require.NoError(t, reqErr)
	req.Header.Set(IdempotencyKeyHeader, "concurrent-key")

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)

	var errResp httputil.ErrorResponse
	unmarshalErr := json.Unmarshal(w.Body.Bytes(), &errResp)
	require.NoError(t, unmarshalErr)
	assert.Contains(t, errResp.Errors.Message, "already being processed")
}

func TestIdempotency_StoreError_ProceedsWithoutIdempotency(t *testing.T) {
	store := newMockStore()
	store.lockErr = errors.New("redis connection refused")

	handlerCalled := false
	r := gin.New()
	r.Use(Idempotency(store))
	r.POST("/test", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(http.StatusCreated, gin.H{"id": "123"})
	})

	body := []byte(`{"name":"test"}`)
	w := httptest.NewRecorder()
	req, reqErr := http.NewRequest("POST", "/test", bytes.NewBuffer(body))
	require.NoError(t, reqErr)
	req.Header.Set(IdempotencyKeyHeader, "failopen-key")

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.True(t, handlerCalled, "handler should be called in fail-open mode")
}

func TestIdempotency_5xxResponse_UnlocksKey(t *testing.T) {
	store := newMockStore()

	r := gin.New()
	r.Use(Idempotency(store))
	r.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server error"})
	})

	body := []byte(`{"name":"test"}`)
	w := httptest.NewRecorder()
	req, reqErr := http.NewRequest("POST", "/test", bytes.NewBuffer(body))
	require.NoError(t, reqErr)
	req.Header.Set(IdempotencyKeyHeader, "5xx-key")

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, 1, store.unlockCalls, "should call Unlock for 5xx responses to allow retry")
	assert.Equal(t, 0, store.completeCalls, "should NOT call Complete for 5xx responses")
}

func TestIdempotency_2xxResponse_CompletesKey(t *testing.T) {
	store := newMockStore()

	r := gin.New()
	r.Use(Idempotency(store))
	r.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"id": "456"})
	})

	body := []byte(`{"name":"test"}`)
	w := httptest.NewRecorder()
	req, reqErr := http.NewRequest("POST", "/test", bytes.NewBuffer(body))
	require.NoError(t, reqErr)
	req.Header.Set(IdempotencyKeyHeader, "2xx-key")

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, 1, store.completeCalls, "should call Complete for 2xx responses")
	assert.Equal(t, 0, store.unlockCalls, "should NOT call Unlock for 2xx responses")

	// Verify the entry is stored as completed
	store.mu.Lock()
	entry := store.entries["idempotency:2xx-key"]
	store.mu.Unlock()
	require.NotNil(t, entry)
	assert.Equal(t, idempotency.StatusCompleted, entry.Status)
	assert.Equal(t, http.StatusCreated, entry.StatusCode)
}

func TestIdempotency_GetError_FailOpen(t *testing.T) {
	store := newMockStore()

	// Pre-populate to make Lock return false (key exists)
	store.mu.Lock()
	store.entries["idempotency:get-err-key"] = &idempotency.Entry{
		Status: idempotency.StatusCompleted,
	}
	store.mu.Unlock()

	// Then make Get fail
	store.getErr = errors.New("redis timeout")

	handlerCalled := false
	r := gin.New()
	r.Use(Idempotency(store))
	r.POST("/test", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(http.StatusCreated, gin.H{"id": "789"})
	})

	body := []byte(`{"name":"test"}`)
	w := httptest.NewRecorder()
	req, reqErr := http.NewRequest("POST", "/test", bytes.NewBuffer(body))
	require.NoError(t, reqErr)
	req.Header.Set(IdempotencyKeyHeader, "get-err-key")

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.True(t, handlerCalled, "should fail-open when Get returns error")
}

func TestIdempotency_WithServiceNameHeader_NamespacesKey(t *testing.T) {
	store := newMockStore()

	r := gin.New()
	r.Use(Idempotency(store))
	r.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"id": "123"})
	})

	body := []byte(`{"name":"test"}`)
	w := httptest.NewRecorder()
	req, reqErr := http.NewRequest("POST", "/test", bytes.NewBuffer(body))
	require.NoError(t, reqErr)
	req.Header.Set(IdempotencyKeyHeader, "ns-key")
	req.Header.Set("X-Service-Name", "my-service")

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	// Verify the key was stored with service namespace
	store.mu.Lock()
	_, exists := store.entries["idempotency:my-service:ns-key"]
	store.mu.Unlock()
	assert.True(t, exists, "key should be namespaced with service name")
}

func TestIdempotency_4xxResponse_CompletesKey(t *testing.T) {
	store := newMockStore()

	r := gin.New()
	r.Use(Idempotency(store))
	r.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
	})

	body := []byte(`{"name":"test"}`)
	w := httptest.NewRecorder()
	req, reqErr := http.NewRequest("POST", "/test", bytes.NewBuffer(body))
	require.NoError(t, reqErr)
	req.Header.Set(IdempotencyKeyHeader, "4xx-key")

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, 1, store.completeCalls, "4xx responses are deterministic and should be stored")
	assert.Equal(t, 0, store.unlockCalls, "should NOT unlock for 4xx responses")
}

func TestBuildIdempotencyKey_WithServiceName(t *testing.T) {
	key := buildIdempotencyKey("my-service", "abc-123")
	assert.Equal(t, "idempotency:my-service:abc-123", key)
}

func TestBuildIdempotencyKey_WithoutServiceName(t *testing.T) {
	key := buildIdempotencyKey("", "abc-123")
	assert.Equal(t, "idempotency:abc-123", key)
}

func TestShouldStoreResponse(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		expected   bool
	}{
		{"200 OK", http.StatusOK, true},
		{"201 Created", http.StatusCreated, true},
		{"400 Bad Request", http.StatusBadRequest, true},
		{"422 Unprocessable Entity", http.StatusUnprocessableEntity, true},
		{"499 Client Closed Request", 499, true},
		{"500 Internal Server Error", http.StatusInternalServerError, false},
		{"502 Bad Gateway", http.StatusBadGateway, false},
		{"503 Service Unavailable", http.StatusServiceUnavailable, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldStoreResponse(tt.statusCode)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBodyFingerprint_Deterministic(t *testing.T) {
	body := []byte(`{"name":"test","value":42}`)
	fp1 := bodyFingerprint(body)
	fp2 := bodyFingerprint(body)
	assert.Equal(t, fp1, fp2, "same body should produce same fingerprint")
}

func TestBodyFingerprint_DifferentBodies(t *testing.T) {
	fp1 := bodyFingerprint([]byte(`{"name":"first"}`))
	fp2 := bodyFingerprint([]byte(`{"name":"second"}`))
	assert.NotEqual(t, fp1, fp2, "different bodies should produce different fingerprints")
}
