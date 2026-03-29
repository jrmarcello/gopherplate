package role

import (
	"context"
	"errors"
	"testing"

	roledomain "bitbucket.org/appmax-space/go-boilerplate/internal/domain/role"
	"bitbucket.org/appmax-space/go-boilerplate/internal/usecases/role/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateUseCase_Execute_Success(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	mockRepo.On("FindByName", mock.Anything, "admin").Return(nil, roledomain.ErrRoleNotFound)
	mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*role.Role")).Return(nil)

	uc := NewCreateUseCase(mockRepo)
	input := dto.CreateInput{
		Name:        "admin",
		Description: "Administrator role",
	}

	// Act
	output, executeErr := uc.Execute(context.Background(), input)

	// Assert
	assert.NoError(t, executeErr)
	assert.NotNil(t, output)
	assert.NotEmpty(t, output.ID)
	assert.NotEmpty(t, output.CreatedAt)
	mockRepo.AssertExpectations(t)
}

func TestCreateUseCase_Execute_DuplicateName(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	existingRole := roledomain.NewRole("admin", "Existing admin role")
	mockRepo.On("FindByName", mock.Anything, "admin").Return(existingRole, nil)

	uc := NewCreateUseCase(mockRepo)
	input := dto.CreateInput{
		Name:        "admin",
		Description: "Administrator role",
	}

	// Act
	output, executeErr := uc.Execute(context.Background(), input)

	// Assert
	assert.Error(t, executeErr)
	assert.Nil(t, output)
	assert.True(t, errors.Is(executeErr, roledomain.ErrDuplicateRoleName))
	mockRepo.AssertNotCalled(t, "Create")
}

func TestCreateUseCase_Execute_FindByNameError(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	mockRepo.On("FindByName", mock.Anything, "admin").
		Return(nil, errors.New("database connection lost"))

	uc := NewCreateUseCase(mockRepo)
	input := dto.CreateInput{
		Name:        "admin",
		Description: "Administrator role",
	}

	// Act
	output, executeErr := uc.Execute(context.Background(), input)

	// Assert
	assert.Error(t, executeErr)
	assert.Nil(t, output)
	assert.Contains(t, executeErr.Error(), "database connection lost")
	mockRepo.AssertNotCalled(t, "Create")
}

func TestCreateUseCase_Execute_RepositoryError(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	mockRepo.On("FindByName", mock.Anything, "admin").Return(nil, roledomain.ErrRoleNotFound)
	mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*role.Role")).
		Return(errors.New("database connection failed"))

	uc := NewCreateUseCase(mockRepo)
	input := dto.CreateInput{
		Name:        "admin",
		Description: "Administrator role",
	}

	// Act
	output, executeErr := uc.Execute(context.Background(), input)

	// Assert
	assert.Error(t, executeErr)
	assert.Nil(t, output)
	assert.Contains(t, executeErr.Error(), "database connection failed")
	mockRepo.AssertExpectations(t)
}
