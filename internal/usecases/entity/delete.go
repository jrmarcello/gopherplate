package entity

import (
	"context"
	"log/slog"
	"time"

	"bitbucket.org/appmax-space/ms-boilerplate-go/internal/domain/entity/vo"
	"bitbucket.org/appmax-space/ms-boilerplate-go/internal/usecases/entity/dto"
	"bitbucket.org/appmax-space/ms-boilerplate-go/internal/usecases/entity/interfaces"
)

// DeleteUseCase implementa o caso de uso de deleção (soft delete) de entity.
type DeleteUseCase struct {
	Repo  interfaces.Repository
	Cache interfaces.Cache
}

// NewDeleteUseCase cria uma nova instância do DeleteUseCase.
func NewDeleteUseCase(repo interfaces.Repository, cache interfaces.Cache) *DeleteUseCase {
	return &DeleteUseCase{
		Repo:  repo,
		Cache: cache,
	}
}

// Execute realiza soft delete de uma entity.
//
// Fluxo:
//  1. Validar ID
//  2. Realizar soft delete (active=false)
//  3. Invalidar cache
func (uc *DeleteUseCase) Execute(ctx context.Context, input dto.DeleteInput) (*dto.DeleteOutput, error) {
	// Validar e converter ID
	id, err := vo.ParseID(input.ID)
	if err != nil {
		return nil, err
	}

	// 2. Realizar soft delete
	if err := uc.Repo.Delete(ctx, id); err != nil {
		return nil, err
	}

	// 3. Invalidar cache
	if uc.Cache != nil {
		cacheKey := "entity:" + input.ID
		if err := uc.Cache.Delete(ctx, cacheKey); err != nil {
			slog.Warn("failed to invalidate cache", "key", cacheKey, "error", err)
		}
	}

	return &dto.DeleteOutput{
		ID:        input.ID,
		DeletedAt: time.Now().Format(time.RFC3339),
	}, nil
}
