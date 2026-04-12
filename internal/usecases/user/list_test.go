package user

import (
	"context"
	"errors"
	"testing"
	"time"

	userdomain "github.com/jrmarcello/go-boilerplate/internal/domain/user"
	"github.com/jrmarcello/go-boilerplate/internal/domain/user/vo"
	"github.com/jrmarcello/go-boilerplate/internal/usecases/user/dto"
	"github.com/jrmarcello/go-boilerplate/pkg/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestListUseCase_Execute_Success(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)

	email1, _ := vo.NewEmail("joao@example.com")
	email2, _ := vo.NewEmail("maria@example.com")

	expectedResult := &userdomain.ListResult{
		Users: []*userdomain.User{
			{
				ID:        vo.NewID(),
				Name:      "João Silva",
				Email:     email1,
				Active:    true,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			{
				ID:        vo.NewID(),
				Name:      "Maria Santos",
				Email:     email2,
				Active:    true,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
		Total: 2,
		Page:  1,
		Limit: 20,
	}

	mockRepo.On("List", mock.Anything, mock.AnythingOfType("user.ListFilter")).Return(expectedResult, nil)

	uc := NewListUseCase(mockRepo)
	input := dto.ListInput{Page: 1, Limit: 20}

	// Act
	output, executeErr := uc.Execute(context.Background(), input)

	// Assert
	assert.NoError(t, executeErr)
	assert.NotNil(t, output)
	assert.Len(t, output.Data, 2)
	assert.Equal(t, 2, output.Pagination.Total)
	assert.Equal(t, 1, output.Pagination.Page)
	mockRepo.AssertExpectations(t)
}

func TestListUseCase_Execute_WithFilters(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)

	email, _ := vo.NewEmail("maria@example.com")
	expectedResult := &userdomain.ListResult{
		Users: []*userdomain.User{
			{
				ID:        vo.NewID(),
				Name:      "Maria Santos",
				Email:     email,
				Active:    true,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
		Total: 1,
		Page:  1,
		Limit: 20,
	}

	mockRepo.On("List", mock.Anything, mock.AnythingOfType("user.ListFilter")).Return(expectedResult, nil)

	uc := NewListUseCase(mockRepo)
	input := dto.ListInput{
		Page:       1,
		Limit:      20,
		Name:       "maria",
		ActiveOnly: true,
	}

	// Act
	output, executeErr := uc.Execute(context.Background(), input)

	// Assert
	assert.NoError(t, executeErr)
	assert.Len(t, output.Data, 1)
	assert.Equal(t, "Maria Santos", output.Data[0].Name)
	mockRepo.AssertExpectations(t)
}

func TestListUseCase_Execute_RepositoryError(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	mockRepo.On("List", mock.Anything, mock.AnythingOfType("user.ListFilter")).
		Return(nil, errors.New("database error"))

	uc := NewListUseCase(mockRepo)
	input := dto.ListInput{Page: 1, Limit: 20}

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

func TestListUseCase_Execute_EmptyResult(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)

	expectedResult := &userdomain.ListResult{
		Users: []*userdomain.User{},
		Total: 0,
		Page:  1,
		Limit: 20,
	}

	mockRepo.On("List", mock.Anything, mock.AnythingOfType("user.ListFilter")).Return(expectedResult, nil)

	uc := NewListUseCase(mockRepo)
	input := dto.ListInput{Page: 1, Limit: 20}

	// Act
	output, executeErr := uc.Execute(context.Background(), input)

	// Assert
	assert.NoError(t, executeErr)
	assert.NotNil(t, output)
	assert.Len(t, output.Data, 0)
	assert.Equal(t, 0, output.Pagination.Total)
	mockRepo.AssertExpectations(t)
}
