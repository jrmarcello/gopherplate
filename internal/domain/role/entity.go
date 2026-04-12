package role

import (
	"time"

	"github.com/jrmarcello/gopherplate/internal/domain/user/vo"
)

// Role é a Entidade principal (Aggregate Root) do domínio de roles.
// Representa um papel/perfil de acesso no sistema.
type Role struct {
	ID          vo.ID
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewRole cria uma nova Role com valores padrão.
func NewRole(name, description string) *Role {
	return &Role{
		ID:          vo.NewID(),
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// UpdateDescription atualiza a descrição da role.
func (r *Role) UpdateDescription(description string) {
	r.Description = description
	r.UpdatedAt = time.Now()
}

// UpdateName atualiza o nome da role.
func (r *Role) UpdateName(name string) {
	r.Name = name
	r.UpdatedAt = time.Now()
}
