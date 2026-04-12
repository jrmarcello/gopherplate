package user

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jrmarcello/go-boilerplate/internal/domain/user/vo"
	"github.com/jrmarcello/go-boilerplate/internal/usecases/user/dto"
	"github.com/jrmarcello/go-boilerplate/internal/usecases/user/interfaces"

	ucshared "github.com/jrmarcello/go-boilerplate/internal/usecases/shared"
	"go.opentelemetry.io/otel/trace"
)

// UpdateUseCase implementa o caso de uso de atualização de user.
type UpdateUseCase struct {
	Repo  interfaces.Repository
	Cache interfaces.Cache
}

// NewUpdateUseCase cria uma nova instância do UpdateUseCase.
func NewUpdateUseCase(repo interfaces.Repository) *UpdateUseCase {
	return &UpdateUseCase{
		Repo: repo,
	}
}

// WithCache sets an optional cache for the use case (builder pattern).
func (uc *UpdateUseCase) WithCache(cache interfaces.Cache) *UpdateUseCase {
	uc.Cache = cache
	return uc
}

// Execute atualiza um user existente.
//
// Fluxo:
//  1. Buscar user existente pelo ID
//  2. Aplicar atualizações parciais
//  3. Persistir alterações
//  4. Invalidar cache
func (uc *UpdateUseCase) Execute(ctx context.Context, input dto.UpdateInput) (*dto.UpdateOutput, error) {
	span := trace.SpanFromContext(ctx)

	// Validar e converter ID
	id, parseErr := vo.ParseID(input.ID)
	if parseErr != nil {
		ucshared.ClassifyError(span, parseErr, updateExpectedErrors, "updating user: invalid ID")
		return nil, userToAppError(parseErr)
	}

	// 1. Buscar user existente
	e, findErr := uc.Repo.FindByID(ctx, id)
	if findErr != nil {
		wrappedErr := fmt.Errorf("updating user: %w", findErr)
		ucshared.ClassifyError(span, findErr, updateExpectedErrors, wrappedErr.Error())
		return nil, userToAppError(findErr)
	}

	// 2. Aplicar atualizações parciais
	if input.Name != nil {
		e.UpdateName(*input.Name)
	}

	if input.Email != nil {
		emailVO, emailErr := vo.NewEmail(*input.Email)
		if emailErr != nil {
			ucshared.ClassifyError(span, emailErr, updateExpectedErrors, "updating user: invalid email")
			return nil, userToAppError(emailErr)
		}
		e.UpdateEmail(emailVO)
	}

	// 3. Persistir alterações
	if saveErr := uc.Repo.Update(ctx, e); saveErr != nil {
		wrappedErr := fmt.Errorf("updating user: %w", saveErr)
		ucshared.ClassifyError(span, saveErr, updateExpectedErrors, wrappedErr.Error())
		return nil, userToAppError(saveErr)
	}

	// 4. Invalidar cache
	if uc.Cache != nil {
		cacheKey := "user:" + input.ID
		if deleteCacheErr := uc.Cache.Delete(ctx, cacheKey); deleteCacheErr != nil {
			slog.Warn("failed to invalidate cache", "key", cacheKey, "error", deleteCacheErr)
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
