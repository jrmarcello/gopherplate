package user

import (
	"context"
	"errors"
	"testing"
	"time"

	userdomain "github.com/jrmarcello/gopherplate/internal/domain/user"
	"github.com/jrmarcello/gopherplate/internal/domain/user/vo"
	"github.com/jrmarcello/gopherplate/internal/usecases/user/dto"
	"github.com/jrmarcello/gopherplate/pkg/apperror"
	"github.com/jrmarcello/gopherplate/pkg/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetUseCase_Execute_Success(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	id := vo.NewID()
	email, _ := vo.NewEmail("joao@example.com")

	expectedEntity := &userdomain.User{
		ID:        id,
		Name:      "João Silva",
		Email:     email,
		Active:    true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	mockRepo.On("FindByID", mock.Anything, id).Return(expectedEntity, nil)

	uc := NewGetUseCase(mockRepo)
	input := dto.GetInput{ID: id.String()}

	// Act
	output, executeErr := uc.Execute(context.Background(), input)

	// Assert
	assert.NoError(t, executeErr)
	assert.NotNil(t, output)
	assert.Equal(t, id.String(), output.ID)
	assert.Equal(t, "João Silva", output.Name)
	assert.Equal(t, "joao@example.com", output.Email)
	mockRepo.AssertExpectations(t)
}

func TestGetUseCase_Execute_NotFound(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	mockRepo.On("FindByID", mock.Anything, mock.AnythingOfType("vo.ID")).
		Return(nil, userdomain.ErrUserNotFound)

	uc := NewGetUseCase(mockRepo)
	input := dto.GetInput{ID: "018e4a2c-6b4d-7000-9410-abcdef123456"}

	// Act
	output, executeErr := uc.Execute(context.Background(), input)

	// Assert
	assert.Error(t, executeErr)
	assert.Nil(t, output)

	var appErr *apperror.AppError
	assert.True(t, errors.As(executeErr, &appErr), "expected *apperror.AppError")
	assert.Equal(t, apperror.CodeNotFound, appErr.Code)
	assert.Equal(t, "user not found", appErr.Message)
	mockRepo.AssertExpectations(t)
}

func TestGetUseCase_Execute_InvalidID(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	uc := NewGetUseCase(mockRepo)
	input := dto.GetInput{ID: "invalid-id"}

	// Act
	output, executeErr := uc.Execute(context.Background(), input)

	// Assert
	assert.Error(t, executeErr)
	assert.Nil(t, output)

	var appErr *apperror.AppError
	assert.True(t, errors.As(executeErr, &appErr), "expected *apperror.AppError")
	assert.Equal(t, apperror.CodeInvalidRequest, appErr.Code)
	assert.Equal(t, "invalid ID", appErr.Message)
	mockRepo.AssertNotCalled(t, "FindByID")
}

func TestGetUseCase_Execute_RepositoryError(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	id := vo.NewID()
	mockRepo.On("FindByID", mock.Anything, id).
		Return(nil, errors.New("database connection failed"))

	uc := NewGetUseCase(mockRepo)
	input := dto.GetInput{ID: id.String()}

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

// ============================================
// CACHE TESTS
// ============================================

func TestGetUseCase_Execute_CacheHit(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	mockCache := new(MockCache)

	id := "018e4a2c-6b4d-7000-9410-abcdef123456"
	cacheKey := "user:" + id

	// Simula cache hit - Get retorna sucesso
	mockCache.On("Get", mock.Anything, cacheKey, mock.AnythingOfType("*dto.GetOutput")).
		Run(func(args mock.Arguments) {
			// Preenche o dest com dados cacheados
			dest := args.Get(2).(*dto.GetOutput)
			dest.ID = id
			dest.Name = "João Silva (cached)"
			dest.Email = "joao@example.com"
		}).
		Return(nil)

	uc := NewGetUseCase(mockRepo).WithCache(mockCache)
	input := dto.GetInput{ID: id}

	// Act
	output, executeErr := uc.Execute(context.Background(), input)

	// Assert
	assert.NoError(t, executeErr)
	assert.NotNil(t, output)
	assert.Equal(t, id, output.ID)
	assert.Equal(t, "João Silva (cached)", output.Name)

	// Repo não deve ser chamado em cache hit
	mockRepo.AssertNotCalled(t, "FindByID")
	mockCache.AssertExpectations(t)
}

func TestGetUseCase_Execute_CacheMiss_ThenSet(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	mockCache := new(MockCache)

	id := vo.NewID()
	cacheKey := "user:" + id.String()
	email, _ := vo.NewEmail("joao@example.com")

	expectedEntity := &userdomain.User{
		ID:        id,
		Name:      "João Silva",
		Email:     email,
		Active:    true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Cache miss
	mockCache.On("Get", mock.Anything, cacheKey, mock.AnythingOfType("*dto.GetOutput")).
		Return(errors.New("cache miss"))

	// Repo retorna dados
	mockRepo.On("FindByID", mock.Anything, id).Return(expectedEntity, nil)

	// Cache set é chamado
	mockCache.On("Set", mock.Anything, cacheKey, mock.AnythingOfType("*dto.GetOutput")).
		Return(nil)

	uc := NewGetUseCase(mockRepo).WithCache(mockCache)
	input := dto.GetInput{ID: id.String()}

	// Act
	output, executeErr := uc.Execute(context.Background(), input)

	// Assert
	assert.NoError(t, executeErr)
	assert.NotNil(t, output)
	assert.Equal(t, id.String(), output.ID)
	assert.Equal(t, "João Silva", output.Name)

	mockRepo.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestGetUseCase_Execute_CacheSetError_StillReturnsData(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	mockCache := new(MockCache)

	id := vo.NewID()
	cacheKey := "user:" + id.String()
	email, _ := vo.NewEmail("joao@example.com")

	expectedEntity := &userdomain.User{
		ID:        id,
		Name:      "João Silva",
		Email:     email,
		Active:    true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Cache miss
	mockCache.On("Get", mock.Anything, cacheKey, mock.AnythingOfType("*dto.GetOutput")).
		Return(errors.New("cache miss"))

	// Repo retorna dados
	mockRepo.On("FindByID", mock.Anything, id).Return(expectedEntity, nil)

	// Cache set falha - mas não deve afetar o retorno
	mockCache.On("Set", mock.Anything, cacheKey, mock.AnythingOfType("*dto.GetOutput")).
		Return(errors.New("redis connection failed"))

	uc := NewGetUseCase(mockRepo).WithCache(mockCache)
	input := dto.GetInput{ID: id.String()}

	// Act
	output, executeErr := uc.Execute(context.Background(), input)

	// Assert - deve retornar dados mesmo com erro no cache
	assert.NoError(t, executeErr)
	assert.NotNil(t, output)
	assert.Equal(t, id.String(), output.ID)

	mockRepo.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

// ============================================
// SINGLEFLIGHT TESTS
// ============================================

func TestGetUseCase_Execute_WithFlight(t *testing.T) {
	t.Run("success: WithFlight configured, repo called, returns data", func(t *testing.T) {
		// Arrange
		mockRepo := new(MockRepository)
		id := vo.NewID()
		email, _ := vo.NewEmail("joao@example.com")

		expectedEntity := &userdomain.User{
			ID:        id,
			Name:      "João Silva",
			Email:     email,
			Active:    true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		mockRepo.On("FindByID", mock.Anything, id).Return(expectedEntity, nil)

		uc := NewGetUseCase(mockRepo).WithFlight(cache.NewFlightGroup())
		input := dto.GetInput{ID: id.String()}

		// Act
		output, executeErr := uc.Execute(context.Background(), input)

		// Assert
		assert.NoError(t, executeErr)
		assert.NotNil(t, output)
		assert.Equal(t, id.String(), output.ID)
		assert.Equal(t, "João Silva", output.Name)
		assert.Equal(t, "joao@example.com", output.Email)
		mockRepo.AssertExpectations(t)
	})

	t.Run("error: WithFlight configured, repo returns not found", func(t *testing.T) {
		// Arrange
		mockRepo := new(MockRepository)
		mockRepo.On("FindByID", mock.Anything, mock.AnythingOfType("vo.ID")).
			Return(nil, userdomain.ErrUserNotFound)

		uc := NewGetUseCase(mockRepo).WithFlight(cache.NewFlightGroup())
		input := dto.GetInput{ID: "018e4a2c-6b4d-7000-9410-abcdef123456"}

		// Act
		output, executeErr := uc.Execute(context.Background(), input)

		// Assert
		assert.Error(t, executeErr)
		assert.Nil(t, output)

		var appErr *apperror.AppError
		assert.True(t, errors.As(executeErr, &appErr), "expected *apperror.AppError")
		assert.Equal(t, apperror.CodeNotFound, appErr.Code)
		mockRepo.AssertExpectations(t)
	})
}
