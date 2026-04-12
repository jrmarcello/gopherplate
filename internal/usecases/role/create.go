package role

import (
	"context"
	"errors"
	"fmt"
	"time"

	roledomain "github.com/jrmarcello/gopherplate/internal/domain/role"
	"github.com/jrmarcello/gopherplate/internal/usecases/role/dto"
	"github.com/jrmarcello/gopherplate/internal/usecases/role/interfaces"
	ucshared "github.com/jrmarcello/gopherplate/internal/usecases/shared"
	"go.opentelemetry.io/otel/trace"
)

// CreateUseCase implementa o caso de uso de criacao de role.
type CreateUseCase struct {
	Repo interfaces.Repository
}

// NewCreateUseCase cria uma nova instancia do CreateUseCase.
func NewCreateUseCase(repo interfaces.Repository) *CreateUseCase {
	return &CreateUseCase{Repo: repo}
}

// Execute executa o caso de uso de criacao de role.
//
// Fluxo:
//  1. Verifica se ja existe uma role com o mesmo nome
//  2. Cria a entidade Role usando a Factory
//  3. Persiste no banco via Repository
//  4. Retorna DTO com ID e timestamp
func (uc *CreateUseCase) Execute(ctx context.Context, input dto.CreateInput) (*dto.CreateOutput, error) {
	span := trace.SpanFromContext(ctx)

	// PASSO 1: Verificar duplicidade de nome
	existingRole, findErr := uc.Repo.FindByName(ctx, input.Name)
	if findErr != nil && !errors.Is(findErr, roledomain.ErrRoleNotFound) {
		wrappedErr := fmt.Errorf("creating role: %w", findErr)
		ucshared.ClassifyError(span, wrappedErr, createExpectedErrors, "creating role")
		return nil, roleToAppError(wrappedErr)
	}
	if existingRole != nil {
		ucshared.ClassifyError(span, roledomain.ErrDuplicateRoleName, createExpectedErrors, "creating role")
		return nil, roleToAppError(roledomain.ErrDuplicateRoleName)
	}

	// PASSO 2: Criar Entidade usando a Factory
	r := roledomain.NewRole(input.Name, input.Description)

	// PASSO 3: Persistir no banco via Repository
	if createErr := uc.Repo.Create(ctx, r); createErr != nil {
		wrappedErr := fmt.Errorf("creating role: %w", createErr)
		ucshared.ClassifyError(span, wrappedErr, createExpectedErrors, "creating role")
		return nil, roleToAppError(wrappedErr)
	}

	// PASSO 4: Retornar Output DTO
	return &dto.CreateOutput{
		ID:        r.ID.String(),
		CreatedAt: r.CreatedAt.Format(time.RFC3339),
	}, nil
}
