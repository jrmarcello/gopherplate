package role

import (
	"context"
	"errors"
	"testing"
	"time"

	roledomain "github.com/jrmarcello/go-boilerplate/internal/domain/role"
	"github.com/jrmarcello/go-boilerplate/internal/domain/user/vo"
	"github.com/jrmarcello/go-boilerplate/internal/usecases/role/dto"
	"github.com/jrmarcello/go-boilerplate/pkg/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestListUseCase_Execute_Success(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)

	expectedResult := &roledomain.ListResult{
		Roles: []*roledomain.Role{
			{
				ID:          vo.NewID(),
				Name:        "admin",
				Description: "Administrator role",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			{
				ID:          vo.NewID(),
				Name:        "editor",
				Description: "Editor role",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
		},
		Total: 2,
		Page:  1,
		Limit: 20,
	}

	mockRepo.On("List", mock.Anything, mock.AnythingOfType("role.ListFilter")).Return(expectedResult, nil)

	uc := NewListUseCase(mockRepo)
	input := dto.ListInput{Page: 1, Limit: 20}

	// Act
	output, listErr := uc.Execute(context.Background(), input)

	// Assert
	assert.NoError(t, listErr)
	assert.NotNil(t, output)
	assert.Len(t, output.Data, 2)
	assert.Equal(t, 2, output.Pagination.Total)
	assert.Equal(t, 1, output.Pagination.Page)
	mockRepo.AssertExpectations(t)
}

func TestListUseCase_Execute_EmptyResult(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)

	expectedResult := &roledomain.ListResult{
		Roles: []*roledomain.Role{},
		Total: 0,
		Page:  1,
		Limit: 20,
	}

	mockRepo.On("List", mock.Anything, mock.AnythingOfType("role.ListFilter")).Return(expectedResult, nil)

	uc := NewListUseCase(mockRepo)
	input := dto.ListInput{Page: 1, Limit: 20}

	// Act
	output, listErr := uc.Execute(context.Background(), input)

	// Assert
	assert.NoError(t, listErr)
	assert.NotNil(t, output)
	assert.Len(t, output.Data, 0)
	assert.Equal(t, 0, output.Pagination.Total)
	mockRepo.AssertExpectations(t)
}

func TestListUseCase_Execute_WithFilter(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)

	expectedResult := &roledomain.ListResult{
		Roles: []*roledomain.Role{
			{
				ID:          vo.NewID(),
				Name:        "admin",
				Description: "Administrator role",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
		},
		Total: 1,
		Page:  1,
		Limit: 20,
	}

	mockRepo.On("List", mock.Anything, mock.AnythingOfType("role.ListFilter")).Return(expectedResult, nil)

	uc := NewListUseCase(mockRepo)
	input := dto.ListInput{
		Page:  1,
		Limit: 20,
		Name:  "admin",
	}

	// Act
	output, listErr := uc.Execute(context.Background(), input)

	// Assert
	assert.NoError(t, listErr)
	assert.Len(t, output.Data, 1)
	assert.Equal(t, "admin", output.Data[0].Name)
	mockRepo.AssertExpectations(t)
}

func TestListUseCase_Execute_RepositoryError(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	mockRepo.On("List", mock.Anything, mock.AnythingOfType("role.ListFilter")).
		Return(nil, errors.New("database error"))

	uc := NewListUseCase(mockRepo)
	input := dto.ListInput{Page: 1, Limit: 20}

	// Act
	output, listErr := uc.Execute(context.Background(), input)

	// Assert
	assert.Error(t, listErr)
	assert.Nil(t, output)

	var appErr *apperror.AppError
	assert.True(t, errors.As(listErr, &appErr))
	assert.Equal(t, apperror.CodeInternalError, appErr.Code)

	mockRepo.AssertExpectations(t)
}
