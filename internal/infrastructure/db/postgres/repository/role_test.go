package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	roledomain "bitbucket.org/appmax-space/go-boilerplate/internal/domain/role"
	"bitbucket.org/appmax-space/go-boilerplate/internal/domain/user/vo"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Unit Tests for internal conversions (não precisam de banco)
// =============================================================================

func TestRoleDB_ToRole_Success(t *testing.T) {
	// Arrange
	now := time.Now().Truncate(time.Microsecond)
	dbModel := roleDB{
		ID:          "018e4a2c-6b4d-7000-9410-abcdef123456",
		Name:        "admin",
		Description: "Administrador do sistema",
		CreatedAt:   now.Add(-24 * time.Hour),
		UpdatedAt:   now,
	}

	// Act
	r, convertErr := dbModel.toRole()

	// Assert
	assert.NoError(t, convertErr)
	assert.NotNil(t, r)
	assert.Equal(t, "018e4a2c-6b4d-7000-9410-abcdef123456", r.ID.String())
	assert.Equal(t, "admin", r.Name)
	assert.Equal(t, "Administrador do sistema", r.Description)
	assert.Equal(t, now.Add(-24*time.Hour), r.CreatedAt)
	assert.Equal(t, now, r.UpdatedAt)
}

func TestRoleDB_ToRole_InvalidID(t *testing.T) {
	// Arrange
	dbModel := roleDB{
		ID:          "invalid-id",
		Name:        "admin",
		Description: "Test",
	}

	// Act
	r, convertErr := dbModel.toRole()

	// Assert
	assert.Error(t, convertErr)
	assert.Nil(t, r)
	assert.Contains(t, convertErr.Error(), "parsing ID")
}

func TestFromDomainRole(t *testing.T) {
	// Arrange
	now := time.Now().Truncate(time.Microsecond)

	domainRole := &roledomain.Role{
		ID:          vo.NewID(),
		Name:        "editor",
		Description: "Editor de conteúdo",
		CreatedAt:   now.Add(-24 * time.Hour),
		UpdatedAt:   now,
	}

	// Act
	dbModel := fromDomainRole(domainRole)

	// Assert
	assert.Equal(t, domainRole.ID.String(), dbModel.ID)
	assert.Equal(t, domainRole.Name, dbModel.Name)
	assert.Equal(t, domainRole.Description, dbModel.Description)
	assert.Equal(t, domainRole.CreatedAt, dbModel.CreatedAt)
	assert.Equal(t, domainRole.UpdatedAt, dbModel.UpdatedAt)
}

func TestFromDomainRole_RoundTrip(t *testing.T) {
	// Teste que podemos converter role -> dbModel -> role sem perda de dados
	now := time.Now().Truncate(time.Microsecond)

	original := &roledomain.Role{
		ID:          vo.NewID(),
		Name:        "viewer",
		Description: "Visualizador somente leitura",
		CreatedAt:   now.Add(-24 * time.Hour),
		UpdatedAt:   now,
	}

	// Convert to DB model
	dbModel := fromDomainRole(original)

	// Convert back to role
	restored, convertErr := dbModel.toRole()

	// Assert
	assert.NoError(t, convertErr)
	assert.Equal(t, original.ID, restored.ID)
	assert.Equal(t, original.Name, restored.Name)
	assert.Equal(t, original.Description, restored.Description)
	// Timestamps devem ser iguais quando truncados para microseconds (Postgres precision)
	assert.Equal(t, original.CreatedAt, restored.CreatedAt)
	assert.Equal(t, original.UpdatedAt, restored.UpdatedAt)
}

// =============================================================================
// Helpers for sqlmock tests
// =============================================================================

func buildTestRole() *roledomain.Role {
	now := time.Now().Truncate(time.Microsecond)

	return &roledomain.Role{
		ID:          vo.NewID(),
		Name:        "admin",
		Description: "Administrador do sistema",
		CreatedAt:   now.Add(-24 * time.Hour),
		UpdatedAt:   now,
	}
}

// =============================================================================
// Unit Tests for RoleRepository with sqlmock
// =============================================================================

// --- Create ------------------------------------------------------------------

func TestRoleRepository_Create(t *testing.T) {
	db, mock, mockErr := sqlmock.New()
	require.NoError(t, mockErr)
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "postgres")
	repo := NewRoleRepository(sqlxDB, sqlxDB)

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec("INSERT INTO roles").
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 1))

		r := buildTestRole()
		createErr := repo.Create(context.Background(), r)

		assert.NoError(t, createErr)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database error", func(t *testing.T) {
		mock.ExpectExec("INSERT INTO roles").
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnError(sql.ErrConnDone)

		r := buildTestRole()
		createErr := repo.Create(context.Background(), r)

		assert.Error(t, createErr)
		assert.ErrorIs(t, createErr, sql.ErrConnDone)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// --- FindByName --------------------------------------------------------------

func TestRoleRepository_FindByName(t *testing.T) {
	db, mock, mockErr := sqlmock.New()
	require.NoError(t, mockErr)
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "postgres")
	repo := NewRoleRepository(sqlxDB, sqlxDB)

	now := time.Now().Truncate(time.Microsecond)
	testID := vo.NewID()

	t.Run("success", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "name", "description", "created_at", "updated_at"}).
			AddRow(testID.String(), "admin", "Administrador do sistema", now, now)

		mock.ExpectQuery("SELECT .+ FROM roles WHERE name").
			WithArgs("admin").
			WillReturnRows(rows)

		result, findErr := repo.FindByName(context.Background(), "admin")

		assert.NoError(t, findErr)
		require.NotNil(t, result)
		assert.Equal(t, testID, result.ID)
		assert.Equal(t, "admin", result.Name)
		assert.Equal(t, "Administrador do sistema", result.Description)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery("SELECT .+ FROM roles WHERE name").
			WithArgs("nonexistent").
			WillReturnError(sql.ErrNoRows)

		result, findErr := repo.FindByName(context.Background(), "nonexistent")

		assert.Nil(t, result)
		assert.ErrorIs(t, findErr, roledomain.ErrRoleNotFound)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database error", func(t *testing.T) {
		mock.ExpectQuery("SELECT .+ FROM roles WHERE name").
			WithArgs("admin").
			WillReturnError(sql.ErrConnDone)

		result, findErr := repo.FindByName(context.Background(), "admin")

		assert.Nil(t, result)
		assert.Error(t, findErr)
		assert.ErrorIs(t, findErr, sql.ErrConnDone)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// --- List --------------------------------------------------------------------

func TestRoleRepository_List(t *testing.T) {
	db, mock, mockErr := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, mockErr)
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "postgres")
	repo := NewRoleRepository(sqlxDB, sqlxDB)

	now := time.Now().Truncate(time.Microsecond)
	testID := vo.NewID()

	t.Run("success with results", func(t *testing.T) {
		mock.ExpectBegin()

		countRows := sqlmock.NewRows([]string{"count"}).AddRow(1)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM roles").
			WillReturnRows(countRows)

		dataRows := sqlmock.NewRows([]string{"id", "name", "description", "created_at", "updated_at"}).
			AddRow(testID.String(), "admin", "Administrador do sistema", now, now)
		mock.ExpectQuery("SELECT .+ FROM roles").
			WillReturnRows(dataRows)

		mock.ExpectCommit()

		filter := roledomain.ListFilter{Page: 1, Limit: 20}
		result, listErr := repo.List(context.Background(), filter)

		assert.NoError(t, listErr)
		require.NotNil(t, result)
		assert.Equal(t, 1, result.Total)
		assert.Equal(t, 1, result.Page)
		assert.Equal(t, 20, result.Limit)
		require.Len(t, result.Roles, 1)
		assert.Equal(t, testID, result.Roles[0].ID)
		assert.Equal(t, "admin", result.Roles[0].Name)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("empty result", func(t *testing.T) {
		mock.ExpectBegin()

		countRows := sqlmock.NewRows([]string{"count"}).AddRow(0)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM roles").
			WillReturnRows(countRows)

		dataRows := sqlmock.NewRows([]string{"id", "name", "description", "created_at", "updated_at"})
		mock.ExpectQuery("SELECT .+ FROM roles").
			WillReturnRows(dataRows)

		mock.ExpectCommit()

		filter := roledomain.ListFilter{Page: 1, Limit: 20}
		result, listErr := repo.List(context.Background(), filter)

		assert.NoError(t, listErr)
		require.NotNil(t, result)
		assert.Equal(t, 0, result.Total)
		assert.Empty(t, result.Roles)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("with name filter", func(t *testing.T) {
		mock.ExpectBegin()

		countRows := sqlmock.NewRows([]string{"count"}).AddRow(1)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM roles WHERE name ILIKE").
			WillReturnRows(countRows)

		dataRows := sqlmock.NewRows([]string{"id", "name", "description", "created_at", "updated_at"}).
			AddRow(testID.String(), "admin", "Administrador do sistema", now, now)
		mock.ExpectQuery("SELECT .+ FROM roles.+WHERE name ILIKE").
			WillReturnRows(dataRows)

		mock.ExpectCommit()

		filter := roledomain.ListFilter{Page: 1, Limit: 20, Name: "admin"}
		result, listErr := repo.List(context.Background(), filter)

		assert.NoError(t, listErr)
		require.NotNil(t, result)
		assert.Equal(t, 1, result.Total)
		require.Len(t, result.Roles, 1)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("transaction begin error", func(t *testing.T) {
		mock.ExpectBegin().WillReturnError(sql.ErrConnDone)

		filter := roledomain.ListFilter{Page: 1, Limit: 20}
		result, listErr := repo.List(context.Background(), filter)

		assert.Nil(t, result)
		assert.Error(t, listErr)
		assert.Contains(t, listErr.Error(), "beginning read transaction")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database error on count query", func(t *testing.T) {
		mock.ExpectBegin()

		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM roles").
			WillReturnError(sql.ErrConnDone)

		mock.ExpectRollback()

		filter := roledomain.ListFilter{Page: 1, Limit: 20}
		result, listErr := repo.List(context.Background(), filter)

		assert.Nil(t, result)
		assert.Error(t, listErr)
		assert.ErrorIs(t, listErr, sql.ErrConnDone)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database error on data query", func(t *testing.T) {
		mock.ExpectBegin()

		countRows := sqlmock.NewRows([]string{"count"}).AddRow(5)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM roles").
			WillReturnRows(countRows)

		mock.ExpectQuery("SELECT .+ FROM roles").
			WillReturnError(sql.ErrConnDone)

		mock.ExpectRollback()

		filter := roledomain.ListFilter{Page: 1, Limit: 20}
		result, listErr := repo.List(context.Background(), filter)

		assert.Nil(t, result)
		assert.Error(t, listErr)
		assert.ErrorIs(t, listErr, sql.ErrConnDone)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("commit error", func(t *testing.T) {
		mock.ExpectBegin()

		countRows := sqlmock.NewRows([]string{"count"}).AddRow(1)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM roles").
			WillReturnRows(countRows)

		dataRows := sqlmock.NewRows([]string{"id", "name", "description", "created_at", "updated_at"}).
			AddRow(testID.String(), "admin", "Administrador do sistema", now, now)
		mock.ExpectQuery("SELECT .+ FROM roles").
			WillReturnRows(dataRows)

		mock.ExpectCommit().WillReturnError(sql.ErrConnDone)

		filter := roledomain.ListFilter{Page: 1, Limit: 20}
		result, listErr := repo.List(context.Background(), filter)

		assert.Nil(t, result)
		assert.Error(t, listErr)
		assert.Contains(t, listErr.Error(), "committing read transaction")
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// --- Delete ------------------------------------------------------------------

func TestRoleRepository_Delete(t *testing.T) {
	db, mock, mockErr := sqlmock.New()
	require.NoError(t, mockErr)
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "postgres")
	repo := NewRoleRepository(sqlxDB, sqlxDB)

	testID := vo.NewID()

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM roles").
			WithArgs(testID.String()).
			WillReturnResult(sqlmock.NewResult(0, 1))

		deleteErr := repo.Delete(context.Background(), testID)

		assert.NoError(t, deleteErr)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("not found - zero rows affected", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM roles").
			WithArgs(testID.String()).
			WillReturnResult(sqlmock.NewResult(0, 0))

		deleteErr := repo.Delete(context.Background(), testID)

		assert.ErrorIs(t, deleteErr, roledomain.ErrRoleNotFound)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database error", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM roles").
			WithArgs(testID.String()).
			WillReturnError(sql.ErrConnDone)

		deleteErr := repo.Delete(context.Background(), testID)

		assert.Error(t, deleteErr)
		assert.ErrorIs(t, deleteErr, sql.ErrConnDone)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
