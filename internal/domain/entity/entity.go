package entity

import (
	"time"

	"bitbucket.org/appmax-space/go-boilerplate/internal/domain/entity/vo"
)

// Entity é a Entidade principal (Aggregate Root) do domínio.
// Estrutura simplificada para o boilerplate.
type Entity struct {
	ID        vo.ID
	Name      string
	Email     vo.Email
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewEntity cria uma nova Entity com valores padrão.
func NewEntity(name string, email vo.Email) *Entity {
	return &Entity{
		ID:        vo.NewID(),
		Name:      name,
		Email:     email,
		Active:    true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// Deactivate desativa a entity (soft delete).
func (e *Entity) Deactivate() {
	e.Active = false
	e.UpdatedAt = time.Now()
}

// Activate reativa a entity.
func (e *Entity) Activate() {
	e.Active = true
	e.UpdatedAt = time.Now()
}

// UpdateEmail atualiza o email da entity.
func (e *Entity) UpdateEmail(email vo.Email) {
	e.Email = email
	e.UpdatedAt = time.Now()
}

// UpdateName atualiza o nome da entity.
func (e *Entity) UpdateName(name string) {
	e.Name = name
	e.UpdatedAt = time.Now()
}
