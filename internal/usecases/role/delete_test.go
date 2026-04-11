package role

import (
	"context"
	"errors"
	"testing"

	roledomain "github.com/jrmarcello/go-boilerplate/internal/domain/role"
	"github.com/jrmarcello/go-boilerplate/internal/domain/user/vo"
	"github.com/jrmarcello/go-boilerplate/internal/usecases/role/dto"
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
	output, deleteErr := uc.Execute(context.Background(), input)

	// Assert
	assert.NoError(t, deleteErr)
	assert.NotNil(t, output)
	assert.Equal(t, id.String(), output.ID)
	assert.NotEmpty(t, output.DeletedAt)
	mockRepo.AssertExpectations(t)
}

func TestDeleteUseCase_Execute_NotFound(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	mockRepo.On("Delete", mock.Anything, mock.AnythingOfType("vo.ID")).
		Return(roledomain.ErrRoleNotFound)

	uc := NewDeleteUseCase(mockRepo)
	input := dto.DeleteInput{ID: "018e4a2c-6b4d-7000-9410-abcdef123456"}

	// Act
	output, deleteErr := uc.Execute(context.Background(), input)

	// Assert
	assert.Error(t, deleteErr)
	assert.Nil(t, output)
	assert.True(t, errors.Is(deleteErr, roledomain.ErrRoleNotFound))
	mockRepo.AssertExpectations(t)
}

func TestDeleteUseCase_Execute_InvalidID(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	uc := NewDeleteUseCase(mockRepo)
	input := dto.DeleteInput{ID: "invalid-id"}

	// Act
	output, deleteErr := uc.Execute(context.Background(), input)

	// Assert
	assert.Error(t, deleteErr)
	assert.Nil(t, output)
	mockRepo.AssertNotCalled(t, "Delete")
}

func TestDeleteUseCase_Execute_RepositoryError(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	mockRepo.On("Delete", mock.Anything, mock.AnythingOfType("vo.ID")).
		Return(errors.New("database error"))

	uc := NewDeleteUseCase(mockRepo)
	input := dto.DeleteInput{ID: "018e4a2c-6b4d-7000-9410-abcdef123456"}

	// Act
	output, deleteErr := uc.Execute(context.Background(), input)

	// Assert
	assert.Error(t, deleteErr)
	assert.Nil(t, output)
	assert.Contains(t, deleteErr.Error(), "database error")
	mockRepo.AssertExpectations(t)
}
