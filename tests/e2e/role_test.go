package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bitbucket.org/appmax-space/go-boilerplate/internal/infrastructure/db/postgres/repository"
	"bitbucket.org/appmax-space/go-boilerplate/internal/infrastructure/web/handler"
	roleuc "bitbucket.org/appmax-space/go-boilerplate/internal/usecases/role"
)

// setupRoleTestRouter configura o router com rotas de role para testes e2e
func setupRoleTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)

	db := GetTestDB()
	repo := repository.NewRoleRepository(db, db)

	createUC := roleuc.NewCreateUseCase(repo)
	listUC := roleuc.NewListUseCase(repo)
	deleteUC := roleuc.NewDeleteUseCase(repo)

	h := handler.NewRoleHandler(createUC, listUC, deleteUC)

	r := gin.New()
	r.Use(gin.Recovery())

	r.POST("/roles", h.Create)
	r.GET("/roles", h.List)
	r.DELETE("/roles/:id", h.Delete)

	return r
}

// cleanupRoles remove todos os roles do banco de teste
func cleanupRoles() error {
	_, execErr := testDB.Exec("DELETE FROM roles")
	return execErr
}

// =============================================================================
// SUCCESS SCENARIOS
// =============================================================================

func TestE2E_CreateRole_Success(t *testing.T) {
	require.NoError(t, cleanupRoles())
	router := setupRoleTestRouter()

	body := `{
		"name": "admin",
		"description": "Administrator role"
	}`

	req := httptest.NewRequest(http.MethodPost, "/roles", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	response := extractData(t, w.Body.Bytes())

	assert.NotEmpty(t, response["id"])
	assert.NotEmpty(t, response["created_at"])

	// Verificar no banco de dados
	var count int
	dbErr := GetTestDB().Get(&count, "SELECT COUNT(*) FROM roles WHERE name = $1", "admin")
	require.NoError(t, dbErr)
	assert.Equal(t, 1, count)
}

func TestE2E_RoleFullCycle(t *testing.T) {
	require.NoError(t, cleanupRoles())
	router := setupRoleTestRouter()

	// 1. Create
	role := map[string]string{
		"name":        "editor",
		"description": "Editor role",
	}
	body, _ := json.Marshal(role)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/roles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var created map[string]interface{}
	unmarshalErr := json.Unmarshal(w.Body.Bytes(), &created)
	require.NoError(t, unmarshalErr)
	createdData := created["data"].(map[string]interface{})
	id := createdData["id"].(string)

	// 2. List - verify it appears
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/roles", nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var listResponse map[string]interface{}
	listErr := json.Unmarshal(w.Body.Bytes(), &listResponse)
	require.NoError(t, listErr)
	data := listResponse["data"].([]interface{})
	assert.Len(t, data, 1)

	firstRole := data[0].(map[string]interface{})
	assert.Equal(t, id, firstRole["id"])
	assert.Equal(t, "editor", firstRole["name"])

	// 3. Delete
	w = httptest.NewRecorder()
	req = httptest.NewRequest("DELETE", "/roles/"+id, nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// 4. List again - verify gone
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/roles", nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var listAfterDelete map[string]interface{}
	listAfterErr := json.Unmarshal(w.Body.Bytes(), &listAfterDelete)
	require.NoError(t, listAfterErr)
	dataAfter := listAfterDelete["data"].([]interface{})
	assert.Len(t, dataAfter, 0)
}

// =============================================================================
// ERROR SCENARIOS
// =============================================================================

func TestE2E_CreateRole_EmptyName(t *testing.T) {
	router := setupRoleTestRouter()

	body := `{
		"name": "",
		"description": "Role with empty name"
	}`

	req := httptest.NewRequest(http.MethodPost, "/roles", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestE2E_CreateRole_DuplicateName(t *testing.T) {
	require.NoError(t, cleanupRoles())
	router := setupRoleTestRouter()

	body := `{
		"name": "admin",
		"description": "Administrator role"
	}`

	// First create - should succeed
	req := httptest.NewRequest(http.MethodPost, "/roles", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	// Second create with same name - should conflict
	req = httptest.NewRequest(http.MethodPost, "/roles", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestE2E_DeleteRole_NotFound(t *testing.T) {
	router := setupRoleTestRouter()

	// UUID v7 valido mas nao existe
	req := httptest.NewRequest("DELETE", "/roles/018e4a2c-6b4d-7000-9410-abcdef123456", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestE2E_DeleteRole_InvalidID(t *testing.T) {
	router := setupRoleTestRouter()

	req := httptest.NewRequest("DELETE", "/roles/not-a-uuid", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// =============================================================================
// PAGINATION
// =============================================================================

func TestE2E_ListRoles_Pagination(t *testing.T) {
	require.NoError(t, cleanupRoles())
	router := setupRoleTestRouter()

	// Create 5 roles with different names
	for i := 1; i <= 5; i++ {
		body, _ := json.Marshal(map[string]string{
			"name":        fmt.Sprintf("role-%d", i),
			"description": fmt.Sprintf("Role number %d", i),
		})
		req := httptest.NewRequest("POST", "/roles", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusCreated, w.Code)
	}

	// List with pagination (page 1, limit 2)
	req := httptest.NewRequest("GET", "/roles?page=1&limit=2", nil)
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
