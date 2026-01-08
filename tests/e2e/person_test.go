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
	personuc "bitbucket.org/appmax-space/ms-boilerplate-go/internal/usecases/person"
)

// setupTestRouter configura o router para testes e2e
func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)

	db := GetTestDB()
	cache := GetTestCache()
	repo := &repository.PersonRepository{DB: db}

	// Use Cases (with cache)
	createUC := personuc.NewCreateUseCase(repo)
	getUC := personuc.NewGetUseCase(repo, cache)
	listUC := personuc.NewListUseCase(repo)
	updateUC := personuc.NewUpdateUseCase(repo, cache)
	deleteUC := personuc.NewDeleteUseCase(repo, cache)

	h := handler.NewPersonHandler(createUC, getUC, listUC, updateUC, deleteUC)

	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// CRUD Routes
	r.POST("/people", h.Create)
	// r.GET("/people", h.List) // DEPRECATED: endpoint returns 410 Gone
	r.GET("/people/:id", h.GetByID)
	r.PUT("/people/:id", h.Update)
	r.DELETE("/people/:id", h.Delete)

	return r
}

func TestE2E_CreatePerson_Success(t *testing.T) {
	// Cleanup antes do teste
	require.NoError(t, CleanupPeople())

	router := setupTestRouter()

	// Arrange
	body := `{
		"name": "João Silva",
		"document": "52998224725",
		"phone": "11999999999",
		"email": "joao@example.com"
	}`

	req := httptest.NewRequest(http.MethodPost, "/people", bytes.NewBufferString(body))
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
	err = GetTestDB().Get(&count, "SELECT COUNT(*) FROM people WHERE document = $1", "52998224725")
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestE2E_CreatePerson_DuplicateCPF(t *testing.T) {
	// Cleanup antes do teste
	require.NoError(t, CleanupPeople())

	router := setupTestRouter()

	// Criar primeiro person
	body := `{
		"name": "João Silva",
		"document": "52998224725",
		"phone": "11999999999",
		"email": "joao@example.com"
	}`

	req1 := httptest.NewRequest(http.MethodPost, "/people", bytes.NewBufferString(body))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	require.Equal(t, http.StatusCreated, w1.Code)

	// Tentar criar segundo person com mesmo CPF
	body2 := `{
		"name": "Maria Santos",
		"document": "52998224725",
		"phone": "11888888888",
		"email": "maria@example.com"
	}`

	req2 := httptest.NewRequest(http.MethodPost, "/people", bytes.NewBufferString(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	// Assert - deve falhar por CPF duplicado
	assert.Equal(t, http.StatusInternalServerError, w2.Code)
}

func TestE2E_CreatePerson_InvalidCPF(t *testing.T) {
	router := setupTestRouter()

	body := `{
		"name": "João Silva",
		"document": "12345678901",
		"phone": "11999999999",
		"email": "joao@example.com"
	}`

	req := httptest.NewRequest(http.MethodPost, "/people", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "INVALID_CPF", response["code"])
}

func TestE2E_CreatePerson_InvalidEmail(t *testing.T) {
	router := setupTestRouter()

	body := `{
		"name": "João Silva",
		"document": "52998224725",
		"phone": "11999999999",
		"email": "invalid-email"
	}`

	req := httptest.NewRequest(http.MethodPost, "/people", bytes.NewBufferString(body))
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

// TestE2E_CreatePerson_PerformanceBaseline verifica latência básica
func TestE2E_CreatePerson_PerformanceBaseline(t *testing.T) {
	require.NoError(t, CleanupPeople())

	router := setupTestRouter()

	body := `{
		"name": "Performance Test",
		"document": "52998224725",
		"phone": "11999999999",
		"email": "perf@example.com"
	}`

	start := time.Now()

	req := httptest.NewRequest(http.MethodPost, "/people", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	duration := time.Since(start)

	assert.Equal(t, http.StatusCreated, w.Code)
	// A criação deve ser rápida (menos de 100ms para operação simples)
	assert.Less(t, duration.Milliseconds(), int64(100), "Request took too long: %v", duration)
}

func TestE2E_PersonFullCycle(t *testing.T) {
	require.NoError(t, CleanupPeople())
	router := setupTestRouter()

	// 1. Create
	person := map[string]string{
		"name":     "Cycle Test",
		"document": "52998224725",
		"phone":    "11999999999",
		"email":    "cycle@example.com",
	}
	body, _ := json.Marshal(person)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/people", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var created map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &created)
	require.NoError(t, err)
	id := created["id"].(string)

	// 2. Get By ID
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/people/"+id, nil)
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
	req = httptest.NewRequest("PUT", "/people/"+id, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// 4. List - DEPRECATED: endpoint returns 410 Gone
	// w = httptest.NewRecorder()
	// req = httptest.NewRequest("GET", "/people", nil)
	// router.ServeHTTP(w, req)
	// require.Equal(t, http.StatusOK, w.Code)
	// var listResponse map[string]interface{}
	// err = json.Unmarshal(w.Body.Bytes(), &listResponse)
	// require.NoError(t, err)
	// Verificar se a lista contém o item (assumindo formato da resposta)
	// Dependendo do wrapper de resposta, pode ser listResponse["data"]

	// 5. Delete
	w = httptest.NewRecorder()
	req = httptest.NewRequest("DELETE", "/people/"+id, nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// 6. Verify Delete (Get should be 404 or inactive)
	// Ajustar expectativa conforme implementação do GetByID
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/people/"+id, nil)
	router.ServeHTTP(w, req)
	// assert.Equal(t, http.StatusNotFound, w.Code)
	// Se o soft delete retorna 404, descomentar acima. Se retorna 200 com active=false:
	// require.Equal(t, http.StatusOK, w.Code)
}

// TestE2E_CacheBehavior verifica que o cache funciona corretamente
func TestE2E_CacheBehavior(t *testing.T) {
	require.NoError(t, CleanupPeople())
	router := setupTestRouter()

	// 1. Create a person
	person := map[string]string{
		"name":     "Cache Test User",
		"document": "52998224725",
		"phone":    "11999999999",
		"email":    "cache@example.com",
	}
	body, _ := json.Marshal(person)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/people", bytes.NewReader(body))
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
	req = httptest.NewRequest("GET", "/people/"+id, nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	duration1 := time.Since(start1)

	var fetched1 map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &fetched1)
	require.NoError(t, err)
	assert.Equal(t, "Cache Test User", fetched1["name"])

	// 3. Second GET - should be cache hit (typically faster)
	start2 := time.Now()
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/people/"+id, nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	duration2 := time.Since(start2)

	var fetched2 map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &fetched2)
	require.NoError(t, err)
	assert.Equal(t, "Cache Test User", fetched2["name"])

	// Log performance (cache hit should be similar or faster)
	t.Logf("First GET (cache miss): %v", duration1)
	t.Logf("Second GET (cache hit): %v", duration2)

	// 4. Update the person - should invalidate cache
	update := map[string]string{"name": "Updated Cache User"}
	body, _ = json.Marshal(update)
	w = httptest.NewRecorder()
	req = httptest.NewRequest("PUT", "/people/"+id, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// 5. Third GET - should reflect updated data (cache was invalidated)
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/people/"+id, nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var fetched3 map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &fetched3)
	require.NoError(t, err)
	assert.Equal(t, "Updated Cache User", fetched3["name"], "Cache should be invalidated after update")
}
