package entity

import (
	"context"
	"time"

	"bitbucket.org/appmax-space/go-boilerplate/internal/domain/entity"
	"bitbucket.org/appmax-space/go-boilerplate/internal/domain/entity/vo"
	"bitbucket.org/appmax-space/go-boilerplate/internal/usecases/entity/dto"
	"bitbucket.org/appmax-space/go-boilerplate/internal/usecases/entity/interfaces"
)

// CreateUseCase implementa o caso de uso de criação de entity.
type CreateUseCase struct {
	Repo interfaces.Repository
}

// NewCreateUseCase cria uma nova instância do CreateUseCase.
func NewCreateUseCase(repo interfaces.Repository) *CreateUseCase {
	return &CreateUseCase{Repo: repo}
}

// Execute executa o caso de uso de criação de entity.
//
// Fluxo:
//  1. Converte primitivos (string) para Value Objects (validação acontece aqui)
//  2. Cria a entidade Entity usando a Factory
//  3. Persiste no banco via Repository
//  4. Retorna DTO com ID e timestamp
func (uc *CreateUseCase) Execute(ctx context.Context, input dto.CreateInput) (*dto.CreateOutput, error) {
	// PASSO 1: Converter primitivos para Value Objects
	emailVO, err := vo.NewEmail(input.Email)
	if err != nil {
		return nil, err
	}

	// PASSO 2: Criar Entidade usando a Factory
	e := entity.NewEntity(input.Name, emailVO)

	// PASSO 3: Persistir no banco via Repository
	if err := uc.Repo.Create(ctx, e); err != nil {
		return nil, err
	}

	// PASSO 4: Retornar Output DTO
	return &dto.CreateOutput{
		ID:        e.ID.String(),
		CreatedAt: e.CreatedAt.Format(time.RFC3339),
	}, nil
}
