package repository

import (
	"testing"
	"time"

	"bitbucket.org/appmax-space/go-boilerplate/internal/domain/entity"
	"bitbucket.org/appmax-space/go-boilerplate/internal/domain/entity/vo"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Unit Tests for internal conversions (não precisam de banco)
// =============================================================================

func TestEntityDB_ToEntity_Success(t *testing.T) {
	// Arrange
	now := time.Now().Truncate(time.Microsecond)
	dbModel := entityDB{
		ID:        "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		Name:      "João Silva",
		Email:     "joao@example.com",
		Active:    true,
		CreatedAt: now.Add(-24 * time.Hour),
		UpdatedAt: now,
	}

	// Act
	entity, err := dbModel.toEntity()

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, entity)
	assert.Equal(t, "01ARZ3NDEKTSV4RRFFQ69G5FAV", entity.ID.String())
	assert.Equal(t, "João Silva", entity.Name)
	assert.Equal(t, "joao@example.com", entity.Email.String())
	assert.True(t, entity.Active)
}

func TestEntityDB_ToEntity_InvalidID(t *testing.T) {
	// Arrange
	dbModel := entityDB{
		ID:    "invalid-id",
		Name:  "Test",
		Email: "test@example.com",
	}

	// Act
	entity, err := dbModel.toEntity()

	// Assert
	assert.Error(t, err)
	assert.Nil(t, entity)
	assert.Contains(t, err.Error(), "erro ao parsear ID")
}

func TestEntityDB_ToEntity_InvalidEmail(t *testing.T) {
	// Arrange
	dbModel := entityDB{
		ID:    "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		Name:  "Test",
		Email: "invalid-email",
	}

	// Act
	entity, err := dbModel.toEntity()

	// Assert
	assert.Error(t, err)
	assert.Nil(t, entity)
	assert.Contains(t, err.Error(), "erro ao parsear email")
}

func TestFromDomainEntity(t *testing.T) {
	// Arrange
	email, _ := vo.NewEmail("joao@example.com")
	now := time.Now().Truncate(time.Microsecond)

	domainEntity := &entity.Entity{
		ID:        vo.NewID(),
		Name:      "João Silva",
		Email:     email,
		Active:    true,
		CreatedAt: now.Add(-24 * time.Hour),
		UpdatedAt: now,
	}

	// Act
	dbModel := fromDomainEntity(domainEntity)

	// Assert
	assert.Equal(t, domainEntity.ID.String(), dbModel.ID)
	assert.Equal(t, domainEntity.Name, dbModel.Name)
	assert.Equal(t, domainEntity.Email.String(), dbModel.Email)
	assert.Equal(t, domainEntity.Active, dbModel.Active)
	assert.Equal(t, domainEntity.CreatedAt, dbModel.CreatedAt)
	assert.Equal(t, domainEntity.UpdatedAt, dbModel.UpdatedAt)
}

func TestFromDomainEntity_InactiveEntity(t *testing.T) {
	// Arrange
	email, _ := vo.NewEmail("inactive@example.com")

	domainEntity := &entity.Entity{
		ID:        vo.NewID(),
		Name:      "Inactive User",
		Email:     email,
		Active:    false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Act
	dbModel := fromDomainEntity(domainEntity)

	// Assert
	assert.False(t, dbModel.Active)
}

func TestFromDomainEntity_RoundTrip(t *testing.T) {
	// Teste que podemos converter entity -> dbModel -> entity sem perda de dados
	email, _ := vo.NewEmail("roundtrip@example.com")
	now := time.Now().Truncate(time.Microsecond)

	original := &entity.Entity{
		ID:        vo.NewID(),
		Name:      "Round Trip Test",
		Email:     email,
		Active:    true,
		CreatedAt: now.Add(-24 * time.Hour),
		UpdatedAt: now,
	}

	// Convert to DB model
	dbModel := fromDomainEntity(original)

	// Convert back to entity
	restored, err := dbModel.toEntity()

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, original.ID, restored.ID)
	assert.Equal(t, original.Name, restored.Name)
	assert.Equal(t, original.Email.String(), restored.Email.String())
	assert.Equal(t, original.Active, restored.Active)
	// Timestamps devem ser iguais quando truncados para microseconds (Postgres precision)
	assert.Equal(t, original.CreatedAt, restored.CreatedAt)
	assert.Equal(t, original.UpdatedAt, restored.UpdatedAt)
}
