package interfaces

import (
	"context"

	roledomain "github.com/jrmarcello/go-boilerplate/internal/domain/role"
	"github.com/jrmarcello/go-boilerplate/internal/domain/user/vo"
)

// Repository define o CONTRATO para persistencia de Role.
//
// Esta e uma INTERFACE definida na camada de USE CASES e IMPLEMENTADA na INFRASTRUCTURE.
// Isso e a essencia da inversao de dependencia (Dependency Inversion Principle).
//
// Beneficios:
//   - Use cases nao sabem nada sobre banco de dados
//   - Facil trocar implementacao (Postgres -> MySQL)
//   - Facil criar mocks para testes
type Repository interface {
	// Create persiste uma nova Role no banco de dados.
	Create(ctx context.Context, r *roledomain.Role) error

	// List retorna uma lista paginada de Roles com filtros opcionais.
	List(ctx context.Context, filter roledomain.ListFilter) (*roledomain.ListResult, error)

	// Delete remove uma Role pelo ID.
	// Retorna ErrRoleNotFound se o ID nao existir.
	Delete(ctx context.Context, id vo.ID) error

	// FindByName busca uma Role pelo nome.
	// Retorna ErrRoleNotFound se nao encontrar.
	FindByName(ctx context.Context, name string) (*roledomain.Role, error)
}
