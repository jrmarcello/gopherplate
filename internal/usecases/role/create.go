package role

import (
	"context"
	"errors"
	"time"

	roledomain "github.com/jrmarcello/go-boilerplate/internal/domain/role"
	"github.com/jrmarcello/go-boilerplate/internal/usecases/role/dto"
	"github.com/jrmarcello/go-boilerplate/internal/usecases/role/interfaces"
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
	// PASSO 1: Verificar duplicidade de nome
	existingRole, findErr := uc.Repo.FindByName(ctx, input.Name)
	if findErr != nil && !errors.Is(findErr, roledomain.ErrRoleNotFound) {
		return nil, findErr
	}
	if existingRole != nil {
		return nil, roledomain.ErrDuplicateRoleName
	}

	// PASSO 2: Criar Entidade usando a Factory
	r := roledomain.NewRole(input.Name, input.Description)

	// PASSO 3: Persistir no banco via Repository
	if createErr := uc.Repo.Create(ctx, r); createErr != nil {
		return nil, createErr
	}

	// PASSO 4: Retornar Output DTO
	return &dto.CreateOutput{
		ID:        r.ID.String(),
		CreatedAt: r.CreatedAt.Format(time.RFC3339),
	}, nil
}
