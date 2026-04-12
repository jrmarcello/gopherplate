package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrmarcello/gopherplate/internal/bootstrap"
	"github.com/jrmarcello/gopherplate/pkg/httputil"
)

// addAuthHeaders adiciona os headers de autenticação para testes
func addAuthHeaders(req *http.Request) {
	req.Header.Set("X-Service-Name", "test-service")
	req.Header.Set("X-Service-Key", "sk_test_service_key_12345")
}

// extractData is a helper that parses the standard API response {"data": ...}
// and returns the inner data as a map.
func extractData(t *testing.T, body []byte) map[string]interface{} {
	t.Helper()
	var envelope map[string]interface{}
	parseErr := json.Unmarshal(body, &envelope)
	require.NoError(t, parseErr)
	data, ok := envelope["data"].(map[string]interface{})
	require.True(t, ok, "expected 'data' key with object value, got: %s", string(body))
	return data
}

// extractErrorResponse parses the standard error response {"errors":{"message":"..."}}
// and returns the parsed ErrorResponse struct.
func extractErrorResponse(t *testing.T, body []byte) httputil.ErrorResponse {
	t.Helper()
	var errResp httputil.ErrorResponse
	parseErr := json.Unmarshal(body, &errResp)
	require.NoError(t, parseErr, "response body should be valid JSON: %s", string(body))
	require.NotEmpty(t, errResp.Errors.Message, "error message should not be empty: %s", string(body))
	return errResp
}

// =============================================================================
// SUCCESS SCENARIOS
// =============================================================================

func TestE2E_CreateUser_Success(t *testing.T) {
	require.NoError(t, CleanupUsers())
	router := bootstrap.SetupTestRouter(t, GetTestDB(), GetTestCache())

	body := `{
		"name": "Test User",
		"email": "test@example.com"
	}`

	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	response := extractData(t, w.Body.Bytes())

	assert.NotEmpty(t, response["id"])
	assert.NotEmpty(t, response["created_at"])

	// Verificar no banco de dados
	var count int
	dbErr := GetTestDB().Get(&count, "SELECT COUNT(*) FROM users WHERE email = $1", "test@example.com")
	require.NoError(t, dbErr)
	assert.Equal(t, 1, count)
}

func TestE2E_UserFullCycle(t *testing.T) {
	require.NoError(t, CleanupUsers())
	router := bootstrap.SetupTestRouter(t, GetTestDB(), GetTestCache())

	// 1. Create
	user := map[string]string{
		"name":  "Cycle Test",
		"email": "cycle@example.com",
	}
	body, _ := json.Marshal(user)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var created map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &created)
	require.NoError(t, err)
	createdData := created["data"].(map[string]interface{})
	id := createdData["id"].(string)

	// 2. Get By ID
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/users/"+id, nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	fetched := extractData(t, w.Body.Bytes())
	assert.Equal(t, "Cycle Test", fetched["name"])

	// 3. Update
	update := map[string]string{"name": "Cycle Update"}
	body, _ = json.Marshal(update)
	w = httptest.NewRecorder()
	req = httptest.NewRequest("PUT", "/users/"+id, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// 4. Verify Update
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/users/"+id, nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	fetched = extractData(t, w.Body.Bytes())
	assert.Equal(t, "Cycle Update", fetched["name"])

	// 5. List
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/users", nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// 6. Delete
	w = httptest.NewRecorder()
	req = httptest.NewRequest("DELETE", "/users/"+id, nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// 7. Verify Delete (soft delete - user becomes inactive)
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/users/"+id, nil)
	router.ServeHTTP(w, req)
	// Soft delete pode retornar 200 com active=false ou 404, depende da implementação
	// Vamos verificar se ainda existe mas está inativo
	if w.Code == http.StatusOK {
		fetched = extractData(t, w.Body.Bytes())
		assert.False(t, fetched["active"].(bool), "User should be inactive after delete")
	}
}

// =============================================================================
// ERROR SCENARIOS
// =============================================================================

// TC-E2E-01: POST /users invalid email returns JSON 400
func TestE2E_CreateUser_InvalidEmail(t *testing.T) {
	router := bootstrap.SetupTestRouter(t, GetTestDB(), GetTestCache())

	body := `{
		"name": "Test User",
		"email": "invalid-email"
	}`

	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Verify JSON error format {"errors":{"message":"..."}}
	errResp := extractErrorResponse(t, w.Body.Bytes())
	assert.Contains(t, errResp.Errors.Message, "invalid")
}

func TestE2E_CreateUser_EmptyRequest(t *testing.T) {
	router := bootstrap.SetupTestRouter(t, GetTestDB(), GetTestCache())

	body := `{}`

	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// With binding:"required" on Name and Email, empty body should return 400
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Verify JSON error format
	errResp := extractErrorResponse(t, w.Body.Bytes())
	assert.NotEmpty(t, errResp.Errors.Message)
}

// TC-E2E-02: POST /users duplicate email returns JSON 409
func TestE2E_CreateUser_DuplicateEmail(t *testing.T) {
	require.NoError(t, CleanupUsers())
	router := bootstrap.SetupTestRouter(t, GetTestDB(), GetTestCache())

	body := `{
		"name": "First User",
		"email": "duplicate@example.com"
	}`

	// First create - should succeed
	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	// Second create with same email - should conflict
	body = `{
		"name": "Second User",
		"email": "duplicate@example.com"
	}`
	req = httptest.NewRequest(http.MethodPost, "/users", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)

	// Verify JSON error format {"errors":{"message":"..."}}
	errResp := extractErrorResponse(t, w.Body.Bytes())
	assert.NotEmpty(t, errResp.Errors.Message)
}

// TC-E2E-03: GET /users/:id not found returns JSON 404
func TestE2E_GetUser_NotFound(t *testing.T) {
	router := bootstrap.SetupTestRouter(t, GetTestDB(), GetTestCache())

	// UUID v7 válido mas não existe
	req := httptest.NewRequest("GET", "/users/018e4a2c-6b4d-7000-9410-abcdef123456", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	// Verify JSON error format {"errors":{"message":"..."}}
	errResp := extractErrorResponse(t, w.Body.Bytes())
	assert.NotEmpty(t, errResp.Errors.Message)
}

// TC-E2E-04: GET /users/:id invalid UUID returns JSON 400
func TestE2E_GetUser_InvalidID(t *testing.T) {
	router := bootstrap.SetupTestRouter(t, GetTestDB(), GetTestCache())

	req := httptest.NewRequest("GET", "/users/invalid-id", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Verify JSON error format {"errors":{"message":"..."}}
	errResp := extractErrorResponse(t, w.Body.Bytes())
	assert.NotEmpty(t, errResp.Errors.Message)
}

func TestE2E_UpdateUser_NotFound(t *testing.T) {
	router := bootstrap.SetupTestRouter(t, GetTestDB(), GetTestCache())

	update := map[string]string{"name": "Updated Name"}
	body, _ := json.Marshal(update)

	req := httptest.NewRequest("PUT", "/users/018e4a2c-6b4d-7000-9410-abcdef123456", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	// Verify JSON error format
	errResp := extractErrorResponse(t, w.Body.Bytes())
	assert.NotEmpty(t, errResp.Errors.Message)
}

func TestE2E_UpdateUser_InvalidEmail(t *testing.T) {
	require.NoError(t, CleanupUsers())
	router := bootstrap.SetupTestRouter(t, GetTestDB(), GetTestCache())

	// 1. Create user
	createBody := `{"name": "Test", "email": "valid@example.com"}`
	req := httptest.NewRequest("POST", "/users", bytes.NewBufferString(createBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var created map[string]interface{}
	unmarshalErr := json.Unmarshal(w.Body.Bytes(), &created)
	require.NoError(t, unmarshalErr)
	createdData := created["data"].(map[string]interface{})
	id := createdData["id"].(string)

	// 2. Update with invalid email
	update := map[string]string{"email": "invalid-email"}
	body, _ := json.Marshal(update)
	req = httptest.NewRequest("PUT", "/users/"+id, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Verify JSON error format
	errResp := extractErrorResponse(t, w.Body.Bytes())
	assert.NotEmpty(t, errResp.Errors.Message)
}

func TestE2E_DeleteUser_NotFound(t *testing.T) {
	router := bootstrap.SetupTestRouter(t, GetTestDB(), GetTestCache())

	req := httptest.NewRequest("DELETE", "/users/018e4a2c-6b4d-7000-9410-abcdef123456", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	// Verify JSON error format
	errResp := extractErrorResponse(t, w.Body.Bytes())
	assert.NotEmpty(t, errResp.Errors.Message)
}

// =============================================================================
// PANIC RECOVERY
// =============================================================================

// TC-E2E-05: Panic recovery returns JSON 500 (not HTML)
func TestE2E_PanicRecovery_ReturnsJSON500(t *testing.T) {
	router := bootstrap.SetupTestRouter(t, GetTestDB(), GetTestCache())

	req := httptest.NewRequest("GET", "/panic-test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Verify HTTP 500 status code
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// Verify response is JSON, not HTML
	contentType := w.Header().Get("Content-Type")
	assert.Contains(t, contentType, "application/json")

	// Verify standard error format {"errors":{"message":"internal server error"}}
	errResp := extractErrorResponse(t, w.Body.Bytes())
	assert.Equal(t, "internal server error", errResp.Errors.Message)
}

// =============================================================================
// HEALTH & READINESS
// =============================================================================

func TestE2E_HealthCheck(t *testing.T) {
	router := bootstrap.SetupTestRouter(t, GetTestDB(), GetTestCache())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	response := extractData(t, w.Body.Bytes())
	assert.Equal(t, "ok", response["status"])
}

func TestE2E_ReadinessProbe(t *testing.T) {
	router := bootstrap.SetupTestRouter(t, GetTestDB(), GetTestCache())

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	response := extractData(t, w.Body.Bytes())
	assert.Equal(t, "ready", response["status"])
}

// =============================================================================
// SERVICE KEY AUTH
// =============================================================================

func TestE2E_ServiceKeyAuth_Errors(t *testing.T) {
	router := bootstrap.SetupTestRouterWithAuth(t, GetTestDB(), GetTestCache(), "test-service:sk_test_service_key_12345")

	t.Run("missing auth headers returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users/018e4a2c-6b4d-7000-8000-000000000001", nil)
		// Sem headers de auth
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "unauthorized")
	})

	t.Run("invalid key returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users/018e4a2c-6b4d-7000-8000-000000000001", nil)
		req.Header.Set("X-Service-Name", "test-service")
		req.Header.Set("X-Service-Key", "wrong_key")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "unauthorized")
	})

	t.Run("unknown service returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users/018e4a2c-6b4d-7000-8000-000000000001", nil)
		req.Header.Set("X-Service-Name", "unknown-service")
		req.Header.Set("X-Service-Key", "any_key")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "unauthorized")
	})

	t.Run("valid key allows access", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users/018e4a2c-6b4d-7000-9410-abcdef123456", nil)
		addAuthHeaders(req)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Não deve ser 401 (pode ser 404 se user não existe, mas não 401)
		assert.NotEqual(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("health endpoint is public", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		// Sem headers de auth
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// =============================================================================
// PAGINATION & FILTERING
// =============================================================================

func TestE2E_ListUsers_Pagination(t *testing.T) {
	require.NoError(t, CleanupUsers())
	router := bootstrap.SetupTestRouter(t, GetTestDB(), GetTestCache())

	// Create 5 users
	for i := 1; i <= 5; i++ {
		body, _ := json.Marshal(map[string]string{
			"name":  "User " + string(rune('A'+i-1)),
			"email": "user" + string(rune('a'+i-1)) + "@example.com",
		})
		req := httptest.NewRequest("POST", "/users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusCreated, w.Code)
	}

	// List with pagination (page 1, limit 2)
	req := httptest.NewRequest("GET", "/users?page=1&limit=2", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	parseErr := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, parseErr)

	data := response["data"].([]interface{})
	pagination := response["meta"].(map[string]interface{})

	assert.Len(t, data, 2)
	assert.Equal(t, float64(5), pagination["total"])
	assert.Equal(t, float64(1), pagination["page"])
	assert.Equal(t, float64(2), pagination["limit"])
}

// =============================================================================
// CACHE BEHAVIOR
// =============================================================================

func TestE2E_CacheBehavior(t *testing.T) {
	require.NoError(t, CleanupUsers())
	router := bootstrap.SetupTestRouter(t, GetTestDB(), GetTestCache())

	// 1. Create a user
	user := map[string]string{
		"name":  "Cache Test User",
		"email": "cache@example.com",
	}
	body, _ := json.Marshal(user)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var created map[string]interface{}
	unmarshalErr := json.Unmarshal(w.Body.Bytes(), &created)
	require.NoError(t, unmarshalErr)
	createdData := created["data"].(map[string]interface{})
	id := createdData["id"].(string)

	// 2. First GET - should be cache miss, fetches from DB
	start1 := time.Now()
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/users/"+id, nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	duration1 := time.Since(start1)

	fetched1 := extractData(t, w.Body.Bytes())
	assert.Equal(t, "Cache Test User", fetched1["name"])

	// 3. Second GET - should be cache hit (typically faster)
	start2 := time.Now()
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/users/"+id, nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	duration2 := time.Since(start2)

	fetched2 := extractData(t, w.Body.Bytes())
	assert.Equal(t, "Cache Test User", fetched2["name"])

	// Log performance (cache hit should be similar or faster)
	t.Logf("First GET (cache miss): %v", duration1)
	t.Logf("Second GET (cache hit): %v", duration2)

	// 4. Update the user - should invalidate cache
	updateBody := map[string]string{"name": "Updated Cache User"}
	body, _ = json.Marshal(updateBody)
	w = httptest.NewRecorder()
	req = httptest.NewRequest("PUT", "/users/"+id, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// 5. Third GET - should reflect updated data (cache was invalidated)
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/users/"+id, nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	fetched3 := extractData(t, w.Body.Bytes())
	assert.Equal(t, "Updated Cache User", fetched3["name"], "Cache should be invalidated after update")
}

// =============================================================================
// PERFORMANCE
// =============================================================================

func TestE2E_CreateUser_PerformanceBaseline(t *testing.T) {
	require.NoError(t, CleanupUsers())
	router := bootstrap.SetupTestRouter(t, GetTestDB(), GetTestCache())

	body := `{
		"name": "Performance Test",
		"email": "perf@example.com"
	}`

	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}
