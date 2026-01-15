package entity

import (
	"context"
	"errors"
	"testing"
	"time"

	"bitbucket.org/appmax-space/ms-boilerplate-go/internal/domain/entity"
	"bitbucket.org/appmax-space/ms-boilerplate-go/internal/domain/entity/vo"
	"bitbucket.org/appmax-space/ms-boilerplate-go/internal/usecases/entity/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestUpdateUseCase_Execute_Success(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	id := vo.NewID()
	email, _ := vo.NewEmail("joao@example.com")

	existingEntity := &entity.Entity{
		ID:        id,
		Name:      "João Silva",
		Email:     email,
		Active:    true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	mockRepo.On("FindByID", mock.Anything, id).Return(existingEntity, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*entity.Entity")).Return(nil)

	uc := NewUpdateUseCase(mockRepo, nil)
	newName := "João Silva Updated"
	input := dto.UpdateInput{
		ID:   id.String(),
		Name: &newName,
	}

	// Act
	output, err := uc.Execute(context.Background(), input)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.Equal(t, "João Silva Updated", output.Name)
	mockRepo.AssertExpectations(t)
}

func TestUpdateUseCase_Execute_NotFound(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	mockRepo.On("FindByID", mock.Anything, mock.AnythingOfType("vo.ID")).
		Return(nil, entity.ErrEntityNotFound)

	uc := NewUpdateUseCase(mockRepo, nil)
	newName := "Updated Name"
	input := dto.UpdateInput{
		ID:   "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		Name: &newName,
	}

	// Act
	output, err := uc.Execute(context.Background(), input)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, output)
	assert.True(t, errors.Is(err, entity.ErrEntityNotFound))
	mockRepo.AssertNotCalled(t, "Update")
}

func TestUpdateUseCase_Execute_InvalidEmail(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	id := vo.NewID()
	email, _ := vo.NewEmail("joao@example.com")

	existingEntity := &entity.Entity{
		ID:        id,
		Name:      "João Silva",
		Email:     email,
		Active:    true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	mockRepo.On("FindByID", mock.Anything, id).Return(existingEntity, nil)

	uc := NewUpdateUseCase(mockRepo, nil)
	invalidEmail := "invalid-email"
	input := dto.UpdateInput{
		ID:    id.String(),
		Email: &invalidEmail,
	}

	// Act
	output, err := uc.Execute(context.Background(), input)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, output)
	assert.True(t, errors.Is(err, vo.ErrInvalidEmail))
	mockRepo.AssertNotCalled(t, "Update")
}

func TestUpdateUseCase_Execute_InvalidID(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	uc := NewUpdateUseCase(mockRepo, nil)
	newName := "Updated Name"
	input := dto.UpdateInput{
		ID:   "invalid-id",
		Name: &newName,
	}

	// Act
	output, err := uc.Execute(context.Background(), input)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, output)
	mockRepo.AssertNotCalled(t, "FindByID")
}

func TestUpdateUseCase_Execute_CacheInvalidation(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	mockCache := new(MockCache)
	id := vo.NewID()
	email, _ := vo.NewEmail("joao@example.com")
	cacheKey := "entity:" + id.String()

	existingEntity := &entity.Entity{
		ID:        id,
		Name:      "João Silva",
		Email:     email,
		Active:    true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	mockRepo.On("FindByID", mock.Anything, id).Return(existingEntity, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*entity.Entity")).Return(nil)
	mockCache.On("Delete", mock.Anything, cacheKey).Return(nil)

	uc := NewUpdateUseCase(mockRepo, mockCache)
	newName := "João Silva Updated"
	input := dto.UpdateInput{
		ID:   id.String(),
		Name: &newName,
	}

	// Act
	output, err := uc.Execute(context.Background(), input)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, output)
	mockCache.AssertCalled(t, "Delete", mock.Anything, cacheKey)
	mockRepo.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}
