package user

import (
	"context"
	"log/slog"
	"time"

	"github.com/jrmarcello/go-boilerplate/internal/domain/user/vo"
	"github.com/jrmarcello/go-boilerplate/internal/usecases/user/dto"
	"github.com/jrmarcello/go-boilerplate/internal/usecases/user/interfaces"
)

// DeleteUseCase implementa o caso de uso de deleção (soft delete) de user.
type DeleteUseCase struct {
	Repo  interfaces.Repository
	Cache interfaces.Cache
}

// NewDeleteUseCase cria uma nova instância do DeleteUseCase.
func NewDeleteUseCase(repo interfaces.Repository) *DeleteUseCase {
	return &DeleteUseCase{
		Repo: repo,
	}
}

// WithCache sets an optional cache for the use case (builder pattern).
func (uc *DeleteUseCase) WithCache(cache interfaces.Cache) *DeleteUseCase {
	uc.Cache = cache
	return uc
}

// Execute realiza soft delete de um user.
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
		cacheKey := "user:" + input.ID
		if err := uc.Cache.Delete(ctx, cacheKey); err != nil {
			slog.Warn("failed to invalidate cache", "key", cacheKey, "error", err)
		}
	}

	return &dto.DeleteOutput{
		ID:        input.ID,
		DeletedAt: time.Now().Format(time.RFC3339),
	}, nil
}
