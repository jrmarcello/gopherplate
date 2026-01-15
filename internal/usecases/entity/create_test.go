package entity

import (
	"context"
	"errors"
	"testing"

	"bitbucket.org/appmax-space/ms-boilerplate-go/internal/domain/entity/vo"
	"bitbucket.org/appmax-space/ms-boilerplate-go/internal/usecases/entity/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateUseCase_Execute_Success(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*entity.Entity")).Return(nil)

	uc := NewCreateUseCase(mockRepo)
	input := dto.CreateInput{
		Name:  "João Silva",
		Email: "joao@example.com",
	}

	// Act
	output, err := uc.Execute(context.Background(), input)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.NotEmpty(t, output.ID)
	assert.NotEmpty(t, output.CreatedAt)
	mockRepo.AssertExpectations(t)
}

func TestCreateUseCase_Execute_InvalidEmail(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	uc := NewCreateUseCase(mockRepo)
	input := dto.CreateInput{
		Name:  "João Silva",
		Email: "invalid-email",
	}

	// Act
	output, err := uc.Execute(context.Background(), input)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, output)
	assert.True(t, errors.Is(err, vo.ErrInvalidEmail), "expected ErrInvalidEmail")
	mockRepo.AssertNotCalled(t, "Create")
}

func TestCreateUseCase_Execute_RepositoryError(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*entity.Entity")).
		Return(errors.New("database connection failed"))

	uc := NewCreateUseCase(mockRepo)
	input := dto.CreateInput{
		Name:  "João Silva",
		Email: "joao@example.com",
	}

	// Act
	output, err := uc.Execute(context.Background(), input)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "database connection failed")
	mockRepo.AssertExpectations(t)
}
