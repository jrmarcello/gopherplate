package user

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jrmarcello/gopherplate/internal/domain/user/vo"
	"github.com/jrmarcello/gopherplate/internal/usecases/user/dto"
	"github.com/jrmarcello/gopherplate/internal/usecases/user/interfaces"

	ucshared "github.com/jrmarcello/gopherplate/internal/usecases/shared"
	"go.opentelemetry.io/otel/trace"
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
	span := trace.SpanFromContext(ctx)

	// Validar e converter ID
	id, parseErr := vo.ParseID(input.ID)
	if parseErr != nil {
		ucshared.ClassifyError(span, parseErr, deleteExpectedErrors, "deleting user: invalid ID")
		return nil, userToAppError(parseErr)
	}

	// 2. Realizar soft delete
	if deleteErr := uc.Repo.Delete(ctx, id); deleteErr != nil {
		wrappedErr := fmt.Errorf("deleting user: %w", deleteErr)
		ucshared.ClassifyError(span, deleteErr, deleteExpectedErrors, wrappedErr.Error())
		return nil, userToAppError(deleteErr)
	}

	// 3. Invalidar cache
	if uc.Cache != nil {
		cacheKey := "user:" + input.ID
		if deleteCacheErr := uc.Cache.Delete(ctx, cacheKey); deleteCacheErr != nil {
			slog.Warn("failed to invalidate cache", "key", cacheKey, "error", deleteCacheErr)
		}
	}

	return &dto.DeleteOutput{ID: input.ID}, nil
}
