package interfaces

import (
	"context"

	"bitbucket.org/appmax-space/ms-boilerplate-go/internal/domain/entity"
	"bitbucket.org/appmax-space/ms-boilerplate-go/internal/domain/entity/vo"
)

// Repository define o CONTRATO para persistência de Entity.
//
// Esta é uma INTERFACE definida na camada de USE CASES e IMPLEMENTADA na INFRASTRUCTURE.
// Isso é a essência da inversão de dependência (Dependency Inversion Principle).
//
// Benefícios:
//   - Use cases não sabem nada sobre banco de dados
//   - Fácil trocar implementação (Postgres → MySQL)
//   - Fácil criar mocks para testes
type Repository interface {
	// Create persiste uma nova Entity no banco de dados.
	Create(ctx context.Context, e *entity.Entity) error

	// FindByID busca uma Entity pelo ID (ULID).
	// Retorna ErrEntityNotFound se não encontrar.
	FindByID(ctx context.Context, id vo.ID) (*entity.Entity, error)

	// FindByEmail busca uma Entity pelo email.
	// Retorna ErrEntityNotFound se não encontrar.
	FindByEmail(ctx context.Context, email vo.Email) (*entity.Entity, error)

	// List retorna uma lista paginada de Entities com filtros opcionais.
	List(ctx context.Context, filter entity.ListFilter) (*entity.ListResult, error)

	// Update atualiza uma Entity existente.
	// Retorna ErrEntityNotFound se o ID não existir.
	Update(ctx context.Context, e *entity.Entity) error

	// Delete realiza soft delete (active=false) de uma Entity.
	// Retorna ErrEntityNotFound se o ID não existir.
	Delete(ctx context.Context, id vo.ID) error
}
