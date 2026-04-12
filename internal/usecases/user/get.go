package user

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	userdomain "github.com/jrmarcello/go-boilerplate/internal/domain/user"
	"github.com/jrmarcello/go-boilerplate/internal/domain/user/vo"
	"github.com/jrmarcello/go-boilerplate/internal/usecases/user/dto"
	"github.com/jrmarcello/go-boilerplate/internal/usecases/user/interfaces"
	"github.com/jrmarcello/go-boilerplate/pkg/cache"

	ucshared "github.com/jrmarcello/go-boilerplate/internal/usecases/shared"
	"go.opentelemetry.io/otel/trace"
)

// GetUseCase implementa o caso de uso de buscar user por ID.
type GetUseCase struct {
	Repo   interfaces.Repository
	Cache  interfaces.Cache   // optional, set via WithCache()
	Flight *cache.FlightGroup // optional, prevents cache stampede
}

// NewGetUseCase cria uma nova instância do GetUseCase.
func NewGetUseCase(repo interfaces.Repository) *GetUseCase {
	return &GetUseCase{
		Repo: repo,
	}
}

// WithCache sets an optional cache for the use case (builder pattern).
func (uc *GetUseCase) WithCache(c interfaces.Cache) *GetUseCase {
	uc.Cache = c
	return uc
}

// WithFlight adds singleflight protection against cache stampede (thundering herd).
func (uc *GetUseCase) WithFlight(fg *cache.FlightGroup) *GetUseCase {
	uc.Flight = fg
	return uc
}

// Execute busca um user pelo ID.
//
// Fluxo com cache:
//  1. Tenta buscar no cache
//  2. Se cache miss, busca no DB
//  3. Armazena no cache para próximas requisições
func (uc *GetUseCase) Execute(ctx context.Context, input dto.GetInput) (*dto.GetOutput, error) {
	span := trace.SpanFromContext(ctx)

	// Validar e converter ID
	id, parseErr := vo.ParseID(input.ID)
	if parseErr != nil {
		ucshared.ClassifyError(span, parseErr, getExpectedErrors, "getting user: invalid ID")
		return nil, userToAppError(parseErr)
	}

	cacheKey := "user:" + input.ID

	// 1. Tentar cache primeiro
	if uc.Cache != nil {
		var cached dto.GetOutput
		if cacheErr := uc.Cache.Get(ctx, cacheKey, &cached); cacheErr == nil {
			slog.Debug("cache hit", "key", cacheKey)
			return &cached, nil
		}
	}

	// 2. Buscar no repositório (cache miss — with singleflight if configured)
	var e *userdomain.User

	if uc.Flight != nil {
		val, flightErr, _ := uc.Flight.Do(input.ID, func() (any, error) {
			return uc.Repo.FindByID(ctx, id)
		})
		if flightErr != nil {
			wrappedErr := fmt.Errorf("getting user: %w", flightErr)
			ucshared.ClassifyError(span, flightErr, getExpectedErrors, wrappedErr.Error())
			return nil, userToAppError(flightErr)
		}
		e = val.(*userdomain.User)
	} else {
		var findErr error
		e, findErr = uc.Repo.FindByID(ctx, id)
		if findErr != nil {
			wrappedErr := fmt.Errorf("getting user: %w", findErr)
			ucshared.ClassifyError(span, findErr, getExpectedErrors, wrappedErr.Error())
			return nil, userToAppError(findErr)
		}
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
		if setCacheErr := uc.Cache.Set(ctx, cacheKey, output); setCacheErr != nil {
			slog.Warn("failed to cache user", "key", cacheKey, "error", setCacheErr)
		}
	}

	return output, nil
}
