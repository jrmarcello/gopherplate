package bootstrap

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db, _, mockErr := sqlmock.New()
	require.NoError(t, mockErr)
	t.Cleanup(func() { _ = db.Close() })
	return sqlx.NewDb(db, "sqlmock")
}

func TestNew_ReturnsPopulatedContainer(t *testing.T) {
	db := newMockDB(t)

	// TC-U-01: bootstrap.New returns Container with all fields non-nil
	c := New(db, db, nil, nil)

	require.NotNil(t, c)
	assert.NotNil(t, c.repos.User, "repos.User")
	assert.NotNil(t, c.repos.Role, "repos.Role")
	assert.NotNil(t, c.userUseCases.Create, "userUseCases.Create")
	assert.NotNil(t, c.userUseCases.Get, "userUseCases.Get")
	assert.NotNil(t, c.userUseCases.List, "userUseCases.List")
	assert.NotNil(t, c.userUseCases.Update, "userUseCases.Update")
	assert.NotNil(t, c.userUseCases.Delete, "userUseCases.Delete")
	assert.NotNil(t, c.roleUseCases.Create, "roleUseCases.Create")
	assert.NotNil(t, c.roleUseCases.List, "roleUseCases.List")
	assert.NotNil(t, c.roleUseCases.Delete, "roleUseCases.Delete")
	assert.NotNil(t, c.Handlers.User, "Handlers.User")
	assert.NotNil(t, c.Handlers.Role, "Handlers.Role")
}

func TestNew_ReposPopulated(t *testing.T) {
	db := newMockDB(t)

	// TC-U-02: Container.repos has all repositories populated
	c := New(db, db, nil, nil)

	assert.NotNil(t, c.repos.User)
	assert.NotNil(t, c.repos.Role)
}

func TestNew_UserUseCasesPopulated(t *testing.T) {
	db := newMockDB(t)

	// TC-U-03: Container.userUseCases has all use cases populated
	c := New(db, db, nil, nil)

	assert.NotNil(t, c.userUseCases.Create)
	assert.NotNil(t, c.userUseCases.Get)
	assert.NotNil(t, c.userUseCases.List)
	assert.NotNil(t, c.userUseCases.Update)
	assert.NotNil(t, c.userUseCases.Delete)
}

func TestNew_RoleUseCasesPopulated(t *testing.T) {
	db := newMockDB(t)

	// TC-U-04: Container.roleUseCases has all use cases populated
	c := New(db, db, nil, nil)

	assert.NotNil(t, c.roleUseCases.Create)
	assert.NotNil(t, c.roleUseCases.List)
	assert.NotNil(t, c.roleUseCases.Delete)
}

func TestNew_HandlersPopulated(t *testing.T) {
	db := newMockDB(t)

	// TC-U-05: Container.Handlers has all handlers populated
	c := New(db, db, nil, nil)

	assert.NotNil(t, c.Handlers.User)
	assert.NotNil(t, c.Handlers.Role)
}
