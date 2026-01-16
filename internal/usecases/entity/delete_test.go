package entity

import (
	"context"
	"errors"
	"testing"

	"bitbucket.org/appmax-space/go-boilerplate/internal/domain/entity"
	"bitbucket.org/appmax-space/go-boilerplate/internal/domain/entity/vo"
	"bitbucket.org/appmax-space/go-boilerplate/internal/usecases/entity/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDeleteUseCase_Execute_Success(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	id := vo.NewID()

	mockRepo.On("Delete", mock.Anything, id).Return(nil)

	uc := NewDeleteUseCase(mockRepo, nil)
	input := dto.DeleteInput{ID: id.String()}

	// Act
	output, err := uc.Execute(context.Background(), input)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.Equal(t, id.String(), output.ID)
	assert.NotEmpty(t, output.DeletedAt)
	mockRepo.AssertExpectations(t)
}

func TestDeleteUseCase_Execute_NotFound(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	mockRepo.On("Delete", mock.Anything, mock.AnythingOfType("vo.ID")).
		Return(entity.ErrEntityNotFound)

	uc := NewDeleteUseCase(mockRepo, nil)
	input := dto.DeleteInput{ID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}

	// Act
	output, err := uc.Execute(context.Background(), input)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, output)
	assert.True(t, errors.Is(err, entity.ErrEntityNotFound))
	mockRepo.AssertExpectations(t)
}

func TestDeleteUseCase_Execute_InvalidID(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	uc := NewDeleteUseCase(mockRepo, nil)
	input := dto.DeleteInput{ID: "invalid-id"}

	// Act
	output, err := uc.Execute(context.Background(), input)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, output)
	mockRepo.AssertNotCalled(t, "Delete")
}

func TestDeleteUseCase_Execute_CacheInvalidation(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	mockCache := new(MockCache)
	id := vo.NewID()
	cacheKey := "entity:" + id.String()

	mockRepo.On("Delete", mock.Anything, id).Return(nil)
	mockCache.On("Delete", mock.Anything, cacheKey).Return(nil)

	uc := NewDeleteUseCase(mockRepo, mockCache)
	input := dto.DeleteInput{ID: id.String()}

	// Act
	output, err := uc.Execute(context.Background(), input)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, output)
	mockCache.AssertCalled(t, "Delete", mock.Anything, cacheKey)
	mockRepo.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestDeleteUseCase_Execute_RepositoryError(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	mockRepo.On("Delete", mock.Anything, mock.AnythingOfType("vo.ID")).
		Return(errors.New("database error"))

	uc := NewDeleteUseCase(mockRepo, nil)
	input := dto.DeleteInput{ID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}

	// Act
	output, err := uc.Execute(context.Background(), input)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "database error")
	mockRepo.AssertExpectations(t)
}
