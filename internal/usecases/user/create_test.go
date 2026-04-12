package user

import (
	"context"
	"errors"
	"testing"

	userdomain "github.com/jrmarcello/go-boilerplate/internal/domain/user"
	"github.com/jrmarcello/go-boilerplate/internal/usecases/user/dto"
	"github.com/jrmarcello/go-boilerplate/pkg/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateUseCase_Execute_Success(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*user.User")).Return(nil)

	uc := NewCreateUseCase(mockRepo)
	input := dto.CreateInput{
		Name:  "João Silva",
		Email: "joao@example.com",
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

func TestCreateUseCase_Execute_InvalidEmail(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	uc := NewCreateUseCase(mockRepo)
	input := dto.CreateInput{
		Name:  "João Silva",
		Email: "invalid-email",
	}

	// Act
	output, executeErr := uc.Execute(context.Background(), input)

	// Assert
	assert.Error(t, executeErr)
	assert.Nil(t, output)

	var appErr *apperror.AppError
	assert.True(t, errors.As(executeErr, &appErr), "expected *apperror.AppError")
	assert.Equal(t, apperror.CodeInvalidRequest, appErr.Code)
	assert.Equal(t, "invalid email", appErr.Message)
	mockRepo.AssertNotCalled(t, "Create")
}

func TestCreateUseCase_Execute_DuplicateEmail(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*user.User")).
		Return(userdomain.ErrDuplicateEmail)

	uc := NewCreateUseCase(mockRepo)
	input := dto.CreateInput{
		Name:  "João Silva",
		Email: "joao@example.com",
	}

	// Act
	output, executeErr := uc.Execute(context.Background(), input)

	// Assert
	assert.Error(t, executeErr)
	assert.Nil(t, output)

	var appErr *apperror.AppError
	assert.True(t, errors.As(executeErr, &appErr), "expected *apperror.AppError")
	assert.Equal(t, apperror.CodeConflict, appErr.Code)
	assert.Equal(t, "email already exists", appErr.Message)
	mockRepo.AssertExpectations(t)
}

func TestCreateUseCase_Execute_RepositoryError(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*user.User")).
		Return(errors.New("database connection failed"))

	uc := NewCreateUseCase(mockRepo)
	input := dto.CreateInput{
		Name:  "João Silva",
		Email: "joao@example.com",
	}

	// Act
	output, executeErr := uc.Execute(context.Background(), input)

	// Assert
	assert.Error(t, executeErr)
	assert.Nil(t, output)

	var appErr *apperror.AppError
	assert.True(t, errors.As(executeErr, &appErr), "expected *apperror.AppError")
	assert.Equal(t, apperror.CodeInternalError, appErr.Code)
	assert.Equal(t, "internal server error", appErr.Message)
	mockRepo.AssertExpectations(t)
}
