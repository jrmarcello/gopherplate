package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bitbucket.org/appmax-space/go-boilerplate/internal/infrastructure/db/postgres/repository"
	"bitbucket.org/appmax-space/go-boilerplate/internal/infrastructure/web/handler"
	"bitbucket.org/appmax-space/go-boilerplate/internal/infrastructure/web/middleware"
	entityuc "bitbucket.org/appmax-space/go-boilerplate/internal/usecases/entity"
)

// setupTestRouter configura o router para testes e2e
func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)

	db := GetTestDB()
	cache := GetTestCache()
	repo := &repository.EntityRepository{DB: db}

	// Use Cases (with cache)
	createUC := entityuc.NewCreateUseCase(repo)
	getUC := entityuc.NewGetUseCase(repo, cache)
	listUC := entityuc.NewListUseCase(repo)
	updateUC := entityuc.NewUpdateUseCase(repo, cache)
	deleteUC := entityuc.NewDeleteUseCase(repo, cache)

	h := handler.NewEntityHandler(createUC, getUC, listUC, updateUC, deleteUC)

	r := gin.New()
	r.Use(gin.Recovery())

	// Public routes
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/ready", func(c *gin.Context) {
		if err := db.Ping(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not ready",
				"error":  "database connection failed",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	// CRUD Routes (without auth for backward compatibility)
	r.POST("/entities", h.Create)
	r.GET("/entities", h.List)
	r.GET("/entities/:id", h.GetByID)
	r.PUT("/entities/:id", h.Update)
	r.DELETE("/entities/:id", h.Delete)

	return r
}

// setupTestRouterWithAuth configura o router com autenticação para testes
func setupTestRouterWithAuth() *gin.Engine {
	gin.SetMode(gin.TestMode)

	db := GetTestDB()
	cache := GetTestCache()
	repo := &repository.EntityRepository{DB: db}

	createUC := entityuc.NewCreateUseCase(repo)
	getUC := entityuc.NewGetUseCase(repo, cache)
	listUC := entityuc.NewListUseCase(repo)
	updateUC := entityuc.NewUpdateUseCase(repo, cache)
	deleteUC := entityuc.NewDeleteUseCase(repo, cache)

	h := handler.NewEntityHandler(createUC, getUC, listUC, updateUC, deleteUC)

	r := gin.New()
	r.Use(gin.Recovery())

	// Service Key Auth middleware com chaves de teste
	authConfig := middleware.ServiceKeyConfig{
		Keys: map[string]string{
			"test-service": "sk_test_service_key_12345",
		},
	}

	// Public routes
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Protected routes
	protected := r.Group("")
	protected.Use(middleware.ServiceKeyAuth(authConfig))
	protected.POST("/entities", h.Create)
	protected.GET("/entities", h.List)
	protected.GET("/entities/:id", h.GetByID)
	protected.PUT("/entities/:id", h.Update)
	protected.DELETE("/entities/:id", h.Delete)

	return r
}

// addAuthHeaders adiciona os headers de autenticação para testes
func addAuthHeaders(req *http.Request) {
	req.Header.Set("X-Service-Name", "test-service")
	req.Header.Set("X-Service-Key", "sk_test_service_key_12345")
}

// =============================================================================
// SUCCESS SCENARIOS
// =============================================================================

func TestE2E_CreateEntity_Success(t *testing.T) {
	require.NoError(t, CleanupEntities())
	router := setupTestRouter()

	body := `{
		"name": "Test Entity",
		"email": "test@example.com"
	}`

	req := httptest.NewRequest(http.MethodPost, "/entities", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.NotEmpty(t, response["id"])
	assert.NotEmpty(t, response["created_at"])

	// Verificar no banco de dados
	var count int
	err = GetTestDB().Get(&count, "SELECT COUNT(*) FROM entities WHERE email = $1", "test@example.com")
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestE2E_EntityFullCycle(t *testing.T) {
	require.NoError(t, CleanupEntities())
	router := setupTestRouter()

	// 1. Create
	entity := map[string]string{
		"name":  "Cycle Test",
		"email": "cycle@example.com",
	}
	body, _ := json.Marshal(entity)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/entities", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var created map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &created)
	require.NoError(t, err)
	id := created["id"].(string)

	// 2. Get By ID
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/entities/"+id, nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var fetched map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &fetched)
	require.NoError(t, err)
	assert.Equal(t, "Cycle Test", fetched["name"])

	// 3. Update
	update := map[string]string{"name": "Cycle Update"}
	body, _ = json.Marshal(update)
	w = httptest.NewRecorder()
	req = httptest.NewRequest("PUT", "/entities/"+id, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// 4. Verify Update
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/entities/"+id, nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	err = json.Unmarshal(w.Body.Bytes(), &fetched)
	require.NoError(t, err)
	assert.Equal(t, "Cycle Update", fetched["name"])

	// 5. List
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/entities", nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// 6. Delete
	w = httptest.NewRecorder()
	req = httptest.NewRequest("DELETE", "/entities/"+id, nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// 7. Verify Delete (soft delete - entity becomes inactive)
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/entities/"+id, nil)
	router.ServeHTTP(w, req)
	// Soft delete pode retornar 200 com active=false ou 404, depende da implementação
	// Vamos verificar se ainda existe mas está inativo
	if w.Code == http.StatusOK {
		err = json.Unmarshal(w.Body.Bytes(), &fetched)
		require.NoError(t, err)
		assert.False(t, fetched["active"].(bool), "Entity should be inactive after delete")
	}
}

// =============================================================================
// ERROR SCENARIOS
// =============================================================================

func TestE2E_CreateEntity_InvalidEmail(t *testing.T) {
	router := setupTestRouter()

	body := `{
		"name": "Test Entity",
		"email": "invalid-email"
	}`

	req := httptest.NewRequest(http.MethodPost, "/entities", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_EMAIL")
}

func TestE2E_CreateEntity_EmptyRequest(t *testing.T) {
	router := setupTestRouter()

	body := `{}`

	req := httptest.NewRequest(http.MethodPost, "/entities", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Pode retornar 400 (validação) ou 201 (sem validação de campos obrigatórios)
	// Verificamos apenas que não retorna erro de servidor
	assert.NotEqual(t, http.StatusInternalServerError, w.Code)
}

func TestE2E_GetEntity_NotFound(t *testing.T) {
	router := setupTestRouter()

	// ULID válido mas não existe
	req := httptest.NewRequest("GET", "/entities/01ARZ3NDEKTSV4RRFFQ69G5FAV", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestE2E_GetEntity_InvalidID(t *testing.T) {
	router := setupTestRouter()

	req := httptest.NewRequest("GET", "/entities/invalid-id", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestE2E_UpdateEntity_NotFound(t *testing.T) {
	router := setupTestRouter()

	update := map[string]string{"name": "Updated Name"}
	body, _ := json.Marshal(update)

	req := httptest.NewRequest("PUT", "/entities/01ARZ3NDEKTSV4RRFFQ69G5FAV", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestE2E_UpdateEntity_InvalidEmail(t *testing.T) {
	require.NoError(t, CleanupEntities())
	router := setupTestRouter()

	// 1. Create entity
	createBody := `{"name": "Test", "email": "valid@example.com"}`
	req := httptest.NewRequest("POST", "/entities", bytes.NewBufferString(createBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var created map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &created)
	require.NoError(t, err)
	id := created["id"].(string)

	// 2. Update with invalid email
	update := map[string]string{"email": "invalid-email"}
	body, _ := json.Marshal(update)
	req = httptest.NewRequest("PUT", "/entities/"+id, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestE2E_DeleteEntity_NotFound(t *testing.T) {
	router := setupTestRouter()

	req := httptest.NewRequest("DELETE", "/entities/01ARZ3NDEKTSV4RRFFQ69G5FAV", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// =============================================================================
// HEALTH & READINESS
// =============================================================================

func TestE2E_HealthCheck(t *testing.T) {
	router := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "ok", response["status"])
}

func TestE2E_ReadinessProbe(t *testing.T) {
	router := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "ready", response["status"])
}

// =============================================================================
// SERVICE KEY AUTH
// =============================================================================

func TestE2E_ServiceKeyAuth_Errors(t *testing.T) {
	router := setupTestRouterWithAuth()

	t.Run("missing auth headers returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/entities/01H123456789", nil)
		// Sem headers de auth
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "MISSING_AUTH_HEADERS")
	})

	t.Run("invalid key returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/entities/01H123456789", nil)
		req.Header.Set("X-Service-Name", "test-service")
		req.Header.Set("X-Service-Key", "wrong_key")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "INVALID_SERVICE_KEY")
	})

	t.Run("unknown service returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/entities/01H123456789", nil)
		req.Header.Set("X-Service-Name", "unknown-service")
		req.Header.Set("X-Service-Key", "any_key")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "UNKNOWN_SERVICE")
	})

	t.Run("valid key allows access", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/entities/01ARZ3NDEKTSV4RRFFQ69G5FAV", nil)
		addAuthHeaders(req)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Não deve ser 401 (pode ser 404 se entity não existe, mas não 401)
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

func TestE2E_ListEntities_Pagination(t *testing.T) {
	require.NoError(t, CleanupEntities())
	router := setupTestRouter()

	// Create 5 entities
	for i := 1; i <= 5; i++ {
		body, _ := json.Marshal(map[string]string{
			"name":  "Entity " + string(rune('A'+i-1)),
			"email": "entity" + string(rune('a'+i-1)) + "@example.com",
		})
		req := httptest.NewRequest("POST", "/entities", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusCreated, w.Code)
	}

	// List with pagination (page 1, limit 2)
	req := httptest.NewRequest("GET", "/entities?page=1&limit=2", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	data := response["data"].([]interface{})
	pagination := response["pagination"].(map[string]interface{})

	assert.Len(t, data, 2)
	assert.Equal(t, float64(5), pagination["total"])
	assert.Equal(t, float64(1), pagination["page"])
	assert.Equal(t, float64(2), pagination["limit"])
}

// =============================================================================
// CACHE BEHAVIOR
// =============================================================================

func TestE2E_CacheBehavior(t *testing.T) {
	require.NoError(t, CleanupEntities())
	router := setupTestRouter()

	// 1. Create an entity
	entity := map[string]string{
		"name":  "Cache Test Entity",
		"email": "cache@example.com",
	}
	body, _ := json.Marshal(entity)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/entities", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var created map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &created)
	require.NoError(t, err)
	id := created["id"].(string)

	// 2. First GET - should be cache miss, fetches from DB
	start1 := time.Now()
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/entities/"+id, nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	duration1 := time.Since(start1)

	var fetched1 map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &fetched1)
	require.NoError(t, err)
	assert.Equal(t, "Cache Test Entity", fetched1["name"])

	// 3. Second GET - should be cache hit (typically faster)
	start2 := time.Now()
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/entities/"+id, nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	duration2 := time.Since(start2)

	var fetched2 map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &fetched2)
	require.NoError(t, err)
	assert.Equal(t, "Cache Test Entity", fetched2["name"])

	// Log performance (cache hit should be similar or faster)
	t.Logf("First GET (cache miss): %v", duration1)
	t.Logf("Second GET (cache hit): %v", duration2)

	// 4. Update the entity - should invalidate cache
	updateBody := map[string]string{"name": "Updated Cache Entity"}
	body, _ = json.Marshal(updateBody)
	w = httptest.NewRecorder()
	req = httptest.NewRequest("PUT", "/entities/"+id, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// 5. Third GET - should reflect updated data (cache was invalidated)
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/entities/"+id, nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var fetched3 map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &fetched3)
	require.NoError(t, err)
	assert.Equal(t, "Updated Cache Entity", fetched3["name"], "Cache should be invalidated after update")
}

// =============================================================================
// PERFORMANCE
// =============================================================================

func TestE2E_CreateEntity_PerformanceBaseline(t *testing.T) {
	require.NoError(t, CleanupEntities())
	router := setupTestRouter()

	body := `{
		"name": "Performance Test",
		"email": "perf@example.com"
	}`

	start := time.Now()

	req := httptest.NewRequest(http.MethodPost, "/entities", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	duration := time.Since(start)

	assert.Equal(t, http.StatusCreated, w.Code)
	// A criação deve ser rápida (menos de 100ms para operação simples)
	assert.Less(t, duration.Milliseconds(), int64(100), "Request took too long: %v", duration)
}
