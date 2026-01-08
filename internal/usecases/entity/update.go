package entity

import (
	"context"
	"log/slog"
	"time"

	"bitbucket.org/appmax-space/ms-boilerplate-go/internal/domain/entity/vo"
	"bitbucket.org/appmax-space/ms-boilerplate-go/internal/usecases/entity/dto"
	"bitbucket.org/appmax-space/ms-boilerplate-go/internal/usecases/entity/interfaces"
)

// UpdateUseCase implementa o caso de uso de atualização de entity.
type UpdateUseCase struct {
	Repo  interfaces.Repository
	Cache interfaces.Cache
}

// NewUpdateUseCase cria uma nova instância do UpdateUseCase.
func NewUpdateUseCase(repo interfaces.Repository, cache interfaces.Cache) *UpdateUseCase {
	return &UpdateUseCase{
		Repo:  repo,
		Cache: cache,
	}
}

// Execute atualiza uma entity existente.
//
// Fluxo:
//  1. Buscar entity existente pelo ID
//  2. Aplicar atualizações parciais
//  3. Persistir alterações
//  4. Invalidar cache
func (uc *UpdateUseCase) Execute(ctx context.Context, input dto.UpdateInput) (*dto.UpdateOutput, error) {
	// Validar e converter ID
	id, err := vo.ParseID(input.ID)
	if err != nil {
		return nil, err
	}

	// 1. Buscar entity existente
	e, err := uc.Repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 2. Aplicar atualizações parciais
	if input.Name != nil {
		e.UpdateName(*input.Name)
	}

	if input.Email != nil {
		emailVO, err := vo.NewEmail(*input.Email)
		if err != nil {
			return nil, err
		}
		e.UpdateEmail(emailVO)
	}

	// 3. Persistir alterações
	if err := uc.Repo.Update(ctx, e); err != nil {
		return nil, err
	}

	// 4. Invalidar cache
	if uc.Cache != nil {
		cacheKey := "entity:" + input.ID
		if err := uc.Cache.Delete(ctx, cacheKey); err != nil {
			slog.Warn("failed to invalidate cache", "key", cacheKey, "error", err)
		}
	}

	return &dto.UpdateOutput{
		ID:        e.ID.String(),
		Name:      e.Name,
		Email:     e.Email.String(),
		Active:    e.Active,
		UpdatedAt: e.UpdatedAt.Format(time.RFC3339),
	}, nil
}
