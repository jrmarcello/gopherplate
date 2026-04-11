package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	userdomain "github.com/jrmarcello/go-boilerplate/internal/domain/user"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/jrmarcello/go-boilerplate/internal/domain/user/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Unit Tests for internal conversions (não precisam de banco)
// =============================================================================

func TestUserDB_ToUser_Success(t *testing.T) {
	// Arrange
	now := time.Now().Truncate(time.Microsecond)
	dbModel := userDB{
		ID:        "018e4a2c-6b4d-7000-9410-abcdef123456",
		Name:      "João Silva",
		Email:     "joao@example.com",
		Active:    true,
		CreatedAt: now.Add(-24 * time.Hour),
		UpdatedAt: now,
	}

	// Act
	u, err := dbModel.toUser()

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, u)
	assert.Equal(t, "018e4a2c-6b4d-7000-9410-abcdef123456", u.ID.String())
	assert.Equal(t, "João Silva", u.Name)
	assert.Equal(t, "joao@example.com", u.Email.String())
	assert.True(t, u.Active)
}

func TestUserDB_ToUser_InvalidID(t *testing.T) {
	// Arrange
	dbModel := userDB{
		ID:    "invalid-id",
		Name:  "Test",
		Email: "test@example.com",
	}

	// Act
	u, err := dbModel.toUser()

	// Assert
	assert.Error(t, err)
	assert.Nil(t, u)
	assert.Contains(t, err.Error(), "parsing ID")
}

func TestUserDB_ToUser_InvalidEmail(t *testing.T) {
	// Arrange
	dbModel := userDB{
		ID:    "018e4a2c-6b4d-7000-9410-abcdef123456",
		Name:  "Test",
		Email: "invalid-email",
	}

	// Act
	u, err := dbModel.toUser()

	// Assert
	assert.Error(t, err)
	assert.Nil(t, u)
	assert.Contains(t, err.Error(), "parsing email")
}

func TestFromDomainUser(t *testing.T) {
	// Arrange
	email, _ := vo.NewEmail("joao@example.com")
	now := time.Now().Truncate(time.Microsecond)

	domainUser := &userdomain.User{
		ID:        vo.NewID(),
		Name:      "João Silva",
		Email:     email,
		Active:    true,
		CreatedAt: now.Add(-24 * time.Hour),
		UpdatedAt: now,
	}

	// Act
	dbModel := fromDomainUser(domainUser)

	// Assert
	assert.Equal(t, domainUser.ID.String(), dbModel.ID)
	assert.Equal(t, domainUser.Name, dbModel.Name)
	assert.Equal(t, domainUser.Email.String(), dbModel.Email)
	assert.Equal(t, domainUser.Active, dbModel.Active)
	assert.Equal(t, domainUser.CreatedAt, dbModel.CreatedAt)
	assert.Equal(t, domainUser.UpdatedAt, dbModel.UpdatedAt)
}

func TestFromDomainUser_InactiveEntity(t *testing.T) {
	// Arrange
	email, _ := vo.NewEmail("inactive@example.com")

	domainUser := &userdomain.User{
		ID:        vo.NewID(),
		Name:      "Inactive User",
		Email:     email,
		Active:    false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Act
	dbModel := fromDomainUser(domainUser)

	// Assert
	assert.False(t, dbModel.Active)
}

func TestFromDomainUser_RoundTrip(t *testing.T) {
	// Teste que podemos converter user -> dbModel -> user sem perda de dados
	email, _ := vo.NewEmail("roundtrip@example.com")
	now := time.Now().Truncate(time.Microsecond)

	original := &userdomain.User{
		ID:        vo.NewID(),
		Name:      "Round Trip Test",
		Email:     email,
		Active:    true,
		CreatedAt: now.Add(-24 * time.Hour),
		UpdatedAt: now,
	}

	// Convert to DB model
	dbModel := fromDomainUser(original)

	// Convert back to user
	restored, err := dbModel.toUser()

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, original.ID, restored.ID)
	assert.Equal(t, original.Name, restored.Name)
	assert.Equal(t, original.Email.String(), restored.Email.String())
	assert.Equal(t, original.Active, restored.Active)
	// Timestamps devem ser iguais quando truncados para microseconds (Postgres precision)
	assert.Equal(t, original.CreatedAt, restored.CreatedAt)
	assert.Equal(t, original.UpdatedAt, restored.UpdatedAt)
}

// =============================================================================
// Helpers for sqlmock tests
// =============================================================================

func buildTestUser() *userdomain.User {
	email, _ := vo.NewEmail("test@example.com")
	now := time.Now().Truncate(time.Microsecond)

	return &userdomain.User{
		ID:        vo.NewID(),
		Name:      "Test User",
		Email:     email,
		Active:    true,
		CreatedAt: now.Add(-24 * time.Hour),
		UpdatedAt: now,
	}
}

// =============================================================================
// Unit Tests for UserRepository with sqlmock
// =============================================================================

// --- Create ------------------------------------------------------------------

func TestUserRepository_Create(t *testing.T) {
	db, mock, mockErr := sqlmock.New()
	require.NoError(t, mockErr)
	defer func() { _ = db.Close() }()

	sqlxDB := sqlx.NewDb(db, "postgres")
	repo := NewUserRepository(sqlxDB, sqlxDB)

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec("INSERT INTO users").
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 1))

		e := buildTestUser()
		createErr := repo.Create(context.Background(), e)

		assert.NoError(t, createErr)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database error", func(t *testing.T) {
		mock.ExpectExec("INSERT INTO users").
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnError(sql.ErrConnDone)

		e := buildTestUser()
		createErr := repo.Create(context.Background(), e)

		assert.Error(t, createErr)
		assert.ErrorIs(t, createErr, sql.ErrConnDone)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// --- FindByID ----------------------------------------------------------------

func TestUserRepository_FindByID(t *testing.T) {
	db, mock, mockErr := sqlmock.New()
	require.NoError(t, mockErr)
	defer func() { _ = db.Close() }()

	sqlxDB := sqlx.NewDb(db, "postgres")
	repo := NewUserRepository(sqlxDB, sqlxDB)

	now := time.Now().Truncate(time.Microsecond)
	testID := vo.NewID()

	t.Run("success", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "name", "email", "active", "created_at", "updated_at"}).
			AddRow(testID.String(), "Test User", "test@example.com", true, now, now)

		mock.ExpectQuery("SELECT .+ FROM users WHERE id").
			WithArgs(testID.String()).
			WillReturnRows(rows)

		result, findErr := repo.FindByID(context.Background(), testID)

		assert.NoError(t, findErr)
		require.NotNil(t, result)
		assert.Equal(t, testID, result.ID)
		assert.Equal(t, "Test User", result.Name)
		assert.Equal(t, "test@example.com", result.Email.String())
		assert.True(t, result.Active)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery("SELECT .+ FROM users WHERE id").
			WithArgs(testID.String()).
			WillReturnError(sql.ErrNoRows)

		result, findErr := repo.FindByID(context.Background(), testID)

		assert.Nil(t, result)
		assert.ErrorIs(t, findErr, userdomain.ErrUserNotFound)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database error", func(t *testing.T) {
		mock.ExpectQuery("SELECT .+ FROM users WHERE id").
			WithArgs(testID.String()).
			WillReturnError(sql.ErrConnDone)

		result, findErr := repo.FindByID(context.Background(), testID)

		assert.Nil(t, result)
		assert.Error(t, findErr)
		assert.ErrorIs(t, findErr, sql.ErrConnDone)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// --- FindByEmail -------------------------------------------------------------

func TestUserRepository_FindByEmail(t *testing.T) {
	db, mock, mockErr := sqlmock.New()
	require.NoError(t, mockErr)
	defer func() { _ = db.Close() }()

	sqlxDB := sqlx.NewDb(db, "postgres")
	repo := NewUserRepository(sqlxDB, sqlxDB)

	now := time.Now().Truncate(time.Microsecond)
	testID := vo.NewID()
	testEmail, _ := vo.NewEmail("test@example.com")

	t.Run("success", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "name", "email", "active", "created_at", "updated_at"}).
			AddRow(testID.String(), "Test User", "test@example.com", true, now, now)

		mock.ExpectQuery("SELECT .+ FROM users WHERE email").
			WithArgs(testEmail.String()).
			WillReturnRows(rows)

		result, findErr := repo.FindByEmail(context.Background(), testEmail)

		assert.NoError(t, findErr)
		require.NotNil(t, result)
		assert.Equal(t, testID, result.ID)
		assert.Equal(t, "Test User", result.Name)
		assert.Equal(t, "test@example.com", result.Email.String())
		assert.True(t, result.Active)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery("SELECT .+ FROM users WHERE email").
			WithArgs(testEmail.String()).
			WillReturnError(sql.ErrNoRows)

		result, findErr := repo.FindByEmail(context.Background(), testEmail)

		assert.Nil(t, result)
		assert.ErrorIs(t, findErr, userdomain.ErrUserNotFound)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database error", func(t *testing.T) {
		mock.ExpectQuery("SELECT .+ FROM users WHERE email").
			WithArgs(testEmail.String()).
			WillReturnError(sql.ErrConnDone)

		result, findErr := repo.FindByEmail(context.Background(), testEmail)

		assert.Nil(t, result)
		assert.Error(t, findErr)
		assert.ErrorIs(t, findErr, sql.ErrConnDone)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// --- List --------------------------------------------------------------------

func TestUserRepository_List(t *testing.T) {
	db, mock, mockErr := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, mockErr)
	defer func() { _ = db.Close() }()

	sqlxDB := sqlx.NewDb(db, "postgres")
	repo := NewUserRepository(sqlxDB, sqlxDB)

	now := time.Now().Truncate(time.Microsecond)
	testID := vo.NewID()

	t.Run("success with results", func(t *testing.T) {
		mock.ExpectBegin()

		countRows := sqlmock.NewRows([]string{"count"}).AddRow(1)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users").
			WillReturnRows(countRows)

		dataRows := sqlmock.NewRows([]string{"id", "name", "email", "active", "created_at", "updated_at"}).
			AddRow(testID.String(), "Test User", "test@example.com", true, now, now)
		mock.ExpectQuery("SELECT .+ FROM users").
			WillReturnRows(dataRows)

		mock.ExpectCommit()

		filter := userdomain.ListFilter{Page: 1, Limit: 20}
		result, listErr := repo.List(context.Background(), filter)

		assert.NoError(t, listErr)
		require.NotNil(t, result)
		assert.Equal(t, 1, result.Total)
		assert.Equal(t, 1, result.Page)
		assert.Equal(t, 20, result.Limit)
		require.Len(t, result.Users, 1)
		assert.Equal(t, testID, result.Users[0].ID)
		assert.Equal(t, "Test User", result.Users[0].Name)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("empty result", func(t *testing.T) {
		mock.ExpectBegin()

		countRows := sqlmock.NewRows([]string{"count"}).AddRow(0)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users").
			WillReturnRows(countRows)

		dataRows := sqlmock.NewRows([]string{"id", "name", "email", "active", "created_at", "updated_at"})
		mock.ExpectQuery("SELECT .+ FROM users").
			WillReturnRows(dataRows)

		mock.ExpectCommit()

		filter := userdomain.ListFilter{Page: 1, Limit: 20}
		result, listErr := repo.List(context.Background(), filter)

		assert.NoError(t, listErr)
		require.NotNil(t, result)
		assert.Equal(t, 0, result.Total)
		assert.Empty(t, result.Users)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("with name filter", func(t *testing.T) {
		mock.ExpectBegin()

		countRows := sqlmock.NewRows([]string{"count"}).AddRow(1)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users WHERE name ILIKE").
			WillReturnRows(countRows)

		dataRows := sqlmock.NewRows([]string{"id", "name", "email", "active", "created_at", "updated_at"}).
			AddRow(testID.String(), "Test User", "test@example.com", true, now, now)
		mock.ExpectQuery("SELECT .+ FROM users.+WHERE name ILIKE").
			WillReturnRows(dataRows)

		mock.ExpectCommit()

		filter := userdomain.ListFilter{Page: 1, Limit: 20, Name: "Test"}
		result, listErr := repo.List(context.Background(), filter)

		assert.NoError(t, listErr)
		require.NotNil(t, result)
		assert.Equal(t, 1, result.Total)
		require.Len(t, result.Users, 1)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("with email filter", func(t *testing.T) {
		mock.ExpectBegin()

		countRows := sqlmock.NewRows([]string{"count"}).AddRow(1)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users WHERE email ILIKE").
			WillReturnRows(countRows)

		dataRows := sqlmock.NewRows([]string{"id", "name", "email", "active", "created_at", "updated_at"}).
			AddRow(testID.String(), "Test User", "test@example.com", true, now, now)
		mock.ExpectQuery("SELECT .+ FROM users.+WHERE email ILIKE").
			WillReturnRows(dataRows)

		mock.ExpectCommit()

		filter := userdomain.ListFilter{Page: 1, Limit: 20, Email: "test@"}
		result, listErr := repo.List(context.Background(), filter)

		assert.NoError(t, listErr)
		require.NotNil(t, result)
		assert.Equal(t, 1, result.Total)
		require.Len(t, result.Users, 1)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("with active only filter", func(t *testing.T) {
		mock.ExpectBegin()

		countRows := sqlmock.NewRows([]string{"count"}).AddRow(1)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users WHERE active = true").
			WillReturnRows(countRows)

		dataRows := sqlmock.NewRows([]string{"id", "name", "email", "active", "created_at", "updated_at"}).
			AddRow(testID.String(), "Active User", "active@example.com", true, now, now)
		mock.ExpectQuery("SELECT .+ FROM users.+WHERE active = true").
			WillReturnRows(dataRows)

		mock.ExpectCommit()

		filter := userdomain.ListFilter{Page: 1, Limit: 20, ActiveOnly: true}
		result, listErr := repo.List(context.Background(), filter)

		assert.NoError(t, listErr)
		require.NotNil(t, result)
		assert.Equal(t, 1, result.Total)
		require.Len(t, result.Users, 1)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("transaction begin error", func(t *testing.T) {
		mock.ExpectBegin().WillReturnError(sql.ErrConnDone)

		filter := userdomain.ListFilter{Page: 1, Limit: 20}
		result, listErr := repo.List(context.Background(), filter)

		assert.Nil(t, result)
		assert.Error(t, listErr)
		assert.Contains(t, listErr.Error(), "beginning read transaction")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database error on count query", func(t *testing.T) {
		mock.ExpectBegin()

		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users").
			WillReturnError(sql.ErrConnDone)

		mock.ExpectRollback()

		filter := userdomain.ListFilter{Page: 1, Limit: 20}
		result, listErr := repo.List(context.Background(), filter)

		assert.Nil(t, result)
		assert.Error(t, listErr)
		assert.ErrorIs(t, listErr, sql.ErrConnDone)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database error on data query", func(t *testing.T) {
		mock.ExpectBegin()

		countRows := sqlmock.NewRows([]string{"count"}).AddRow(5)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users").
			WillReturnRows(countRows)

		mock.ExpectQuery("SELECT .+ FROM users").
			WillReturnError(sql.ErrConnDone)

		mock.ExpectRollback()

		filter := userdomain.ListFilter{Page: 1, Limit: 20}
		result, listErr := repo.List(context.Background(), filter)

		assert.Nil(t, result)
		assert.Error(t, listErr)
		assert.ErrorIs(t, listErr, sql.ErrConnDone)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// --- Update ------------------------------------------------------------------

func TestUserRepository_Update(t *testing.T) {
	db, mock, mockErr := sqlmock.New()
	require.NoError(t, mockErr)
	defer func() { _ = db.Close() }()

	sqlxDB := sqlx.NewDb(db, "postgres")
	repo := NewUserRepository(sqlxDB, sqlxDB)

	t.Run("success", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE users SET").
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		e := buildTestUser()
		updateErr := repo.Update(context.Background(), e)

		assert.NoError(t, updateErr)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("not found - zero rows affected", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE users SET").
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectRollback()

		e := buildTestUser()
		updateErr := repo.Update(context.Background(), e)

		assert.ErrorIs(t, updateErr, userdomain.ErrUserNotFound)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database error on exec", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE users SET").
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnError(sql.ErrConnDone)
		mock.ExpectRollback()

		e := buildTestUser()
		updateErr := repo.Update(context.Background(), e)

		assert.Error(t, updateErr)
		assert.ErrorIs(t, updateErr, sql.ErrConnDone)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("transaction begin error", func(t *testing.T) {
		mock.ExpectBegin().WillReturnError(sql.ErrConnDone)

		e := buildTestUser()
		updateErr := repo.Update(context.Background(), e)

		assert.Error(t, updateErr)
		assert.Contains(t, updateErr.Error(), "beginning transaction")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("transaction commit error", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE users SET").
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit().WillReturnError(sql.ErrConnDone)

		e := buildTestUser()
		updateErr := repo.Update(context.Background(), e)

		assert.Error(t, updateErr)
		assert.Contains(t, updateErr.Error(), "committing transaction")
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// --- Delete ------------------------------------------------------------------

func TestUserRepository_Delete(t *testing.T) {
	db, mock, mockErr := sqlmock.New()
	require.NoError(t, mockErr)
	defer func() { _ = db.Close() }()

	sqlxDB := sqlx.NewDb(db, "postgres")
	repo := NewUserRepository(sqlxDB, sqlxDB)

	testID := vo.NewID()

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec("UPDATE users SET").
			WithArgs(sqlmock.AnyArg(), testID.String()).
			WillReturnResult(sqlmock.NewResult(0, 1))

		deleteErr := repo.Delete(context.Background(), testID)

		assert.NoError(t, deleteErr)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("not found - zero rows affected", func(t *testing.T) {
		mock.ExpectExec("UPDATE users SET").
			WithArgs(sqlmock.AnyArg(), testID.String()).
			WillReturnResult(sqlmock.NewResult(0, 0))

		deleteErr := repo.Delete(context.Background(), testID)

		assert.ErrorIs(t, deleteErr, userdomain.ErrUserNotFound)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database error", func(t *testing.T) {
		mock.ExpectExec("UPDATE users SET").
			WithArgs(sqlmock.AnyArg(), testID.String()).
			WillReturnError(sql.ErrConnDone)

		deleteErr := repo.Delete(context.Background(), testID)

		assert.Error(t, deleteErr)
		assert.ErrorIs(t, deleteErr, sql.ErrConnDone)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
