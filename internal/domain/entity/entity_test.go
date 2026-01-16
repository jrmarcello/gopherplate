package entity

import (
	"testing"

	"bitbucket.org/appmax-space/go-boilerplate/internal/domain/entity/vo"
	"github.com/stretchr/testify/assert"
)

func TestNewEntity(t *testing.T) {
	email, _ := vo.NewEmail("test@example.com")

	entity := NewEntity("Test Name", email)

	assert.NotEmpty(t, entity.ID)
	assert.Equal(t, "Test Name", entity.Name)
	assert.Equal(t, "test@example.com", entity.Email.String())
	assert.True(t, entity.Active)
	assert.NotZero(t, entity.CreatedAt)
	assert.NotZero(t, entity.UpdatedAt)
}

func TestEntity_Deactivate(t *testing.T) {
	email, _ := vo.NewEmail("test@example.com")
	entity := NewEntity("Test Name", email)

	entity.Deactivate()

	assert.False(t, entity.Active)
}

func TestEntity_Activate(t *testing.T) {
	email, _ := vo.NewEmail("test@example.com")
	entity := NewEntity("Test Name", email)
	entity.Deactivate()

	entity.Activate()

	assert.True(t, entity.Active)
}

func TestEntity_UpdateName(t *testing.T) {
	email, _ := vo.NewEmail("test@example.com")
	entity := NewEntity("Old Name", email)

	entity.UpdateName("New Name")

	assert.Equal(t, "New Name", entity.Name)
}

func TestEntity_UpdateEmail(t *testing.T) {
	email, _ := vo.NewEmail("old@example.com")
	entity := NewEntity("Test Name", email)
	newEmail, _ := vo.NewEmail("new@example.com")

	entity.UpdateEmail(newEmail)

	assert.Equal(t, "new@example.com", entity.Email.String())
}
