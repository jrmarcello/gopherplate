package entity

import (
	"context"
	"log/slog"
	"time"

	"bitbucket.org/appmax-space/go-boilerplate/internal/domain/entity/vo"
	"bitbucket.org/appmax-space/go-boilerplate/internal/usecases/entity/dto"
	"bitbucket.org/appmax-space/go-boilerplate/internal/usecases/entity/interfaces"
)

// GetUseCase implementa o caso de uso de buscar entity por ID.
type GetUseCase struct {
	Repo  interfaces.Repository
	Cache interfaces.Cache // opcional, pode ser nil
}

// NewGetUseCase cria uma nova instância do GetUseCase.
func NewGetUseCase(repo interfaces.Repository, cache interfaces.Cache) *GetUseCase {
	return &GetUseCase{
		Repo:  repo,
		Cache: cache,
	}
}

// Execute busca uma entity pelo ID.
//
// Fluxo com cache:
//  1. Tenta buscar no cache
//  2. Se cache miss, busca no DB
//  3. Armazena no cache para próximas requisições
func (uc *GetUseCase) Execute(ctx context.Context, input dto.GetInput) (*dto.GetOutput, error) {
	// Validar e converter ID
	id, err := vo.ParseID(input.ID)
	if err != nil {
		return nil, err
	}

	cacheKey := "entity:" + input.ID

	// 1. Tentar cache primeiro
	if uc.Cache != nil {
		var cached dto.GetOutput
		if cacheErr := uc.Cache.Get(ctx, cacheKey, &cached); cacheErr == nil {
			slog.Debug("cache hit", "key", cacheKey)
			return &cached, nil
		}
	}

	// 2. Buscar no repositório (cache miss)
	e, err := uc.Repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 3. Converter para DTO de saída
	output := &dto.GetOutput{
		ID:        e.ID.String(),
		Name:      e.Name,
		Email:     e.Email.String(),
		Active:    e.Active,
		CreatedAt: e.CreatedAt.Format(time.RFC3339),
		UpdatedAt: e.UpdatedAt.Format(time.RFC3339),
	}

	// 4. Armazenar no cache
	if uc.Cache != nil {
		if err := uc.Cache.Set(ctx, cacheKey, output); err != nil {
			slog.Warn("failed to cache entity", "key", cacheKey, "error", err)
		}
	}

	return output, nil
}
