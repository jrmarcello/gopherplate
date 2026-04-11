package user

import (
	"context"
	"errors"
	"testing"

	userdomain "github.com/jrmarcello/go-boilerplate/internal/domain/user"

	"github.com/jrmarcello/go-boilerplate/internal/domain/user/vo"
	"github.com/jrmarcello/go-boilerplate/internal/usecases/user/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDeleteUseCase_Execute_Success(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	id := vo.NewID()

	mockRepo.On("Delete", mock.Anything, id).Return(nil)

	uc := NewDeleteUseCase(mockRepo)
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
		Return(userdomain.ErrUserNotFound)

	uc := NewDeleteUseCase(mockRepo)
	input := dto.DeleteInput{ID: "018e4a2c-6b4d-7000-9410-abcdef123456"}

	// Act
	output, err := uc.Execute(context.Background(), input)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, output)
	assert.True(t, errors.Is(err, userdomain.ErrUserNotFound))
	mockRepo.AssertExpectations(t)
}

func TestDeleteUseCase_Execute_InvalidID(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	uc := NewDeleteUseCase(mockRepo)
	input := dto.DeleteInput{ID: "invalid-id"}

	// Act
	output, err := uc.Execute(context.Background(), input)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, output)
	mockRepo.AssertNotCalled(t, "Delete")
}

func TestDeleteUseCase_Execute_CacheDeleteError_StillSucceeds(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	mockCache := new(MockCache)
	id := vo.NewID()
	cacheKey := "user:" + id.String()

	mockRepo.On("Delete", mock.Anything, id).Return(nil)
	mockCache.On("Delete", mock.Anything, cacheKey).Return(errors.New("redis connection refused"))

	uc := NewDeleteUseCase(mockRepo).WithCache(mockCache)
	input := dto.DeleteInput{ID: id.String()}

	// Act
	output, deleteErr := uc.Execute(context.Background(), input)

	// Assert — delete succeeds even though cache delete failed
	assert.NoError(t, deleteErr)
	assert.NotNil(t, output)
	assert.Equal(t, id.String(), output.ID)
	mockCache.AssertCalled(t, "Delete", mock.Anything, cacheKey)
}

func TestDeleteUseCase_Execute_CacheInvalidation(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	mockCache := new(MockCache)
	id := vo.NewID()
	cacheKey := "user:" + id.String()

	mockRepo.On("Delete", mock.Anything, id).Return(nil)
	mockCache.On("Delete", mock.Anything, cacheKey).Return(nil)

	uc := NewDeleteUseCase(mockRepo).WithCache(mockCache)
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

	uc := NewDeleteUseCase(mockRepo)
	input := dto.DeleteInput{ID: "018e4a2c-6b4d-7000-9410-abcdef123456"}

	// Act
	output, err := uc.Execute(context.Background(), input)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "database error")
	mockRepo.AssertExpectations(t)
}
