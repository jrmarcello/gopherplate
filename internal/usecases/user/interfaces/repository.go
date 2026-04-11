package interfaces

import (
	"context"

	userdomain "github.com/jrmarcello/go-boilerplate/internal/domain/user"
	"github.com/jrmarcello/go-boilerplate/internal/domain/user/vo"
)

// Repository define o CONTRATO para persistência de User.
//
// Esta é uma INTERFACE definida na camada de USE CASES e IMPLEMENTADA na INFRASTRUCTURE.
// Isso é a essência da inversão de dependência (Dependency Inversion Principle).
//
// Benefícios:
//   - Use cases não sabem nada sobre banco de dados
//   - Fácil trocar implementação (Postgres → MySQL)
//   - Fácil criar mocks para testes
type Repository interface {
	// Create persiste um novo User no banco de dados.
	Create(ctx context.Context, e *userdomain.User) error

	// FindByID busca um User pelo ID (UUID v7).
	// Retorna ErrUserNotFound se não encontrar.
	FindByID(ctx context.Context, id vo.ID) (*userdomain.User, error)

	// FindByEmail busca um User pelo email.
	// Retorna ErrUserNotFound se não encontrar.
	FindByEmail(ctx context.Context, email vo.Email) (*userdomain.User, error)

	// List retorna uma lista paginada de Users com filtros opcionais.
	List(ctx context.Context, filter userdomain.ListFilter) (*userdomain.ListResult, error)

	// Update atualiza um User existente.
	// Retorna ErrUserNotFound se o ID não existir.
	Update(ctx context.Context, e *userdomain.User) error

	// Delete realiza soft delete (active=false) de um User.
	// Retorna ErrUserNotFound se o ID não existir.
	Delete(ctx context.Context, id vo.ID) error
}
