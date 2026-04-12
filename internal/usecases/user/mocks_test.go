package user

import (
	"context"

	userdomain "github.com/jrmarcello/gopherplate/internal/domain/user"

	"github.com/jrmarcello/gopherplate/internal/domain/user/vo"
	"github.com/stretchr/testify/mock"
)

// =============================================================================
// MockRepository - Mock do repositório de User para testes unitários
// =============================================================================

// MockRepository implementa a interface Repository para testes
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) Create(ctx context.Context, e *userdomain.User) error {
	args := m.Called(ctx, e)
	return args.Error(0)
}

func (m *MockRepository) FindByID(ctx context.Context, id vo.ID) (*userdomain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*userdomain.User), args.Error(1)
}

func (m *MockRepository) FindByEmail(ctx context.Context, email vo.Email) (*userdomain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*userdomain.User), args.Error(1)
}

func (m *MockRepository) List(ctx context.Context, filter userdomain.ListFilter) (*userdomain.ListResult, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*userdomain.ListResult), args.Error(1)
}

func (m *MockRepository) Update(ctx context.Context, e *userdomain.User) error {
	args := m.Called(ctx, e)
	return args.Error(0)
}

func (m *MockRepository) Delete(ctx context.Context, id vo.ID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// =============================================================================
// MockCache - Mock da interface de Cache para testes unitários
// =============================================================================

// MockCache implementa a interface Cache para testes
type MockCache struct {
	mock.Mock
}

func (m *MockCache) Get(ctx context.Context, key string, dest interface{}) error {
	args := m.Called(ctx, key, dest)
	return args.Error(0)
}

func (m *MockCache) Set(ctx context.Context, key string, value interface{}) error {
	args := m.Called(ctx, key, value)
	return args.Error(0)
}

func (m *MockCache) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockCache) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockCache) Close() error {
	args := m.Called()
	return args.Error(0)
}
