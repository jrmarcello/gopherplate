package user

import (
	"context"
	"log/slog"
	"time"

	userdomain "bitbucket.org/appmax-space/go-boilerplate/internal/domain/user"
	"bitbucket.org/appmax-space/go-boilerplate/internal/domain/user/vo"
	"bitbucket.org/appmax-space/go-boilerplate/internal/usecases/user/dto"
	"bitbucket.org/appmax-space/go-boilerplate/internal/usecases/user/interfaces"
	"bitbucket.org/appmax-space/go-boilerplate/pkg/cache"
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
func (uc *GetUseCase) WithCache(cache interfaces.Cache) *GetUseCase {
	uc.Cache = cache
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
	// Validar e converter ID
	id, err := vo.ParseID(input.ID)
	if err != nil {
		return nil, err
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
			return nil, flightErr
		}
		e = val.(*userdomain.User)
	} else {
		var findErr error
		e, findErr = uc.Repo.FindByID(ctx, id)
		if findErr != nil {
			return nil, findErr
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
		if err := uc.Cache.Set(ctx, cacheKey, output); err != nil {
			slog.Warn("failed to cache user", "key", cacheKey, "error", err)
		}
	}

	return output, nil
}
