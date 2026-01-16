package entity

import (
	"context"
	"errors"
	"testing"
	"time"

	"bitbucket.org/appmax-space/go-boilerplate/internal/domain/entity"
	"bitbucket.org/appmax-space/go-boilerplate/internal/domain/entity/vo"
	"bitbucket.org/appmax-space/go-boilerplate/internal/usecases/entity/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetUseCase_Execute_Success(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	id := vo.NewID()
	email, _ := vo.NewEmail("joao@example.com")

	expectedEntity := &entity.Entity{
		ID:        id,
		Name:      "João Silva",
		Email:     email,
		Active:    true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	mockRepo.On("FindByID", mock.Anything, id).Return(expectedEntity, nil)

	uc := NewGetUseCase(mockRepo, nil)
	input := dto.GetInput{ID: id.String()}

	// Act
	output, err := uc.Execute(context.Background(), input)

	// Assert
	assert.NoError(t, err)
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
		Return(nil, entity.ErrEntityNotFound)

	uc := NewGetUseCase(mockRepo, nil)
	input := dto.GetInput{ID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}

	// Act
	output, err := uc.Execute(context.Background(), input)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, output)
	assert.True(t, errors.Is(err, entity.ErrEntityNotFound))
	mockRepo.AssertExpectations(t)
}

func TestGetUseCase_Execute_InvalidID(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	uc := NewGetUseCase(mockRepo, nil)
	input := dto.GetInput{ID: "invalid-id"}

	// Act
	output, err := uc.Execute(context.Background(), input)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, output)
	mockRepo.AssertNotCalled(t, "FindByID")
}

// ============================================
// CACHE TESTS
// ============================================

func TestGetUseCase_Execute_CacheHit(t *testing.T) {
	// Arrange
	mockRepo := new(MockRepository)
	mockCache := new(MockCache)

	id := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	cacheKey := "entity:" + id

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

	uc := NewGetUseCase(mockRepo, mockCache)
	input := dto.GetInput{ID: id}

	// Act
	output, err := uc.Execute(context.Background(), input)

	// Assert
	assert.NoError(t, err)
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
	cacheKey := "entity:" + id.String()
	email, _ := vo.NewEmail("joao@example.com")

	expectedEntity := &entity.Entity{
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

	uc := NewGetUseCase(mockRepo, mockCache)
	input := dto.GetInput{ID: id.String()}

	// Act
	output, err := uc.Execute(context.Background(), input)

	// Assert
	assert.NoError(t, err)
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
	cacheKey := "entity:" + id.String()
	email, _ := vo.NewEmail("joao@example.com")

	expectedEntity := &entity.Entity{
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

	uc := NewGetUseCase(mockRepo, mockCache)
	input := dto.GetInput{ID: id.String()}

	// Act
	output, err := uc.Execute(context.Background(), input)

	// Assert - deve retornar dados mesmo com erro no cache
	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.Equal(t, id.String(), output.ID)

	mockRepo.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}
