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

	"bitbucket.org/appmax-space/ms-boilerplate-go/internal/infrastructure/db/postgres/repository"
	"bitbucket.org/appmax-space/ms-boilerplate-go/internal/infrastructure/web/handler"
	entityuc "bitbucket.org/appmax-space/ms-boilerplate-go/internal/usecases/entity"
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

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// CRUD Routes
	r.POST("/entities", h.Create)
	r.GET("/entities", h.List)
	r.GET("/entities/:id", h.GetByID)
	r.PUT("/entities/:id", h.Update)
	r.DELETE("/entities/:id", h.Delete)

	return r
}

func TestE2E_CreateEntity_Success(t *testing.T) {
	// Cleanup antes do teste
	require.NoError(t, CleanupEntities())

	router := setupTestRouter()

	// Arrange
	body := `{
		"name": "Test Entity",
		"email": "test@example.com"
	}`

	req := httptest.NewRequest(http.MethodPost, "/entities", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act
	router.ServeHTTP(w, req)

	// Assert
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
}

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

// TestE2E_CreateEntity_PerformanceBaseline verifica latência básica
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

	// 4. List
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/entities", nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// 5. Delete
	w = httptest.NewRecorder()
	req = httptest.NewRequest("DELETE", "/entities/"+id, nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

// TestE2E_CacheBehavior verifica que o cache funciona corretamente
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
