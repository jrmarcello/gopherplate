package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"bitbucket.org/appmax-space/go-boilerplate/internal/domain/entity"
	"bitbucket.org/appmax-space/go-boilerplate/internal/domain/entity/vo"
	"github.com/jmoiron/sqlx"
)

// entityDB é o modelo de banco de dados (Data Model).
//
// Por que ter um modelo separado da entidade de domínio?
//   - Nomes de colunas podem ser diferentes dos campos da entidade
//   - Permite adicionar campos específicos do banco (audit, soft delete)
//   - Desacopla mudanças de schema de mudanças de domínio
type entityDB struct {
	ID        string    `db:"id"`
	Name      string    `db:"name"`
	Email     string    `db:"email"`
	Active    bool      `db:"active"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (e *entityDB) toEntity() (*entity.Entity, error) {
	id, err := vo.ParseID(e.ID)
	if err != nil {
		return nil, fmt.Errorf("erro ao parsear ID: %w", err)
	}

	email, err := vo.NewEmail(e.Email)
	if err != nil {
		return nil, fmt.Errorf("erro ao parsear email: %w", err)
	}

	return &entity.Entity{
		ID:        id,
		Name:      e.Name,
		Email:     email,
		Active:    e.Active,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
	}, nil
}

func fromDomainEntity(e *entity.Entity) entityDB {
	return entityDB{
		ID:        e.ID.String(),
		Name:      e.Name,
		Email:     e.Email.String(),
		Active:    e.Active,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
	}
}

// EntityRepository implementa a interface Repository para Entity.
type EntityRepository struct {
	DB *sqlx.DB
}

func (r *EntityRepository) Create(ctx context.Context, e *entity.Entity) error {
	query := `
		INSERT INTO entities (
			id, name, email, active, created_at, updated_at
		) VALUES (
			:id, :name, :email, :active, :created_at, :updated_at
		)
	`

	dbModel := fromDomainEntity(e)
	_, err := r.DB.NamedExecContext(ctx, query, dbModel)
	return err
}

func (r *EntityRepository) FindByID(ctx context.Context, id vo.ID) (*entity.Entity, error) {
	query := `
		SELECT id, name, email, active, created_at, updated_at
		FROM entities
		WHERE id = $1
	`

	var dbModel entityDB
	err := r.DB.GetContext(ctx, &dbModel, query, id.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, entity.ErrEntityNotFound
		}
		return nil, err
	}

	return dbModel.toEntity()
}

func (r *EntityRepository) FindByEmail(ctx context.Context, email vo.Email) (*entity.Entity, error) {
	query := `
		SELECT id, name, email, active, created_at, updated_at
		FROM entities
		WHERE email = $1
	`

	var dbModel entityDB
	err := r.DB.GetContext(ctx, &dbModel, query, email.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, entity.ErrEntityNotFound
		}
		return nil, err
	}

	return dbModel.toEntity()
}

func (r *EntityRepository) List(ctx context.Context, filter entity.ListFilter) (*entity.ListResult, error) {
	filter.Normalize()

	// Construir query dinâmica com filtros
	var conditions []string
	args := make(map[string]interface{})

	if filter.ActiveOnly {
		conditions = append(conditions, "active = true")
	}
	if filter.Name != "" {
		conditions = append(conditions, "name ILIKE :name")
		args["name"] = "%" + filter.Name + "%"
	}
	if filter.Email != "" {
		conditions = append(conditions, "email ILIKE :email")
		args["email"] = "%" + filter.Email + "%"
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Query para contar total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM entities %s", whereClause)
	var total int

	countQuery, countArgs, err := sqlx.Named(countQuery, args)
	if err != nil {
		return nil, err
	}
	countQuery = r.DB.Rebind(countQuery)

	err = r.DB.GetContext(ctx, &total, countQuery, countArgs...)
	if err != nil {
		return nil, err
	}

	// Query para buscar dados paginados
	args["limit"] = filter.Limit
	args["offset"] = filter.Offset()

	dataQuery := fmt.Sprintf(`
		SELECT id, name, email, active, created_at, updated_at
		FROM entities
		%s
		ORDER BY created_at DESC
		LIMIT :limit OFFSET :offset
	`, whereClause)

	dataQuery, dataArgs, err := sqlx.Named(dataQuery, args)
	if err != nil {
		return nil, err
	}
	dataQuery = r.DB.Rebind(dataQuery)

	var dbModels []entityDB
	err = r.DB.SelectContext(ctx, &dbModels, dataQuery, dataArgs...)
	if err != nil {
		return nil, err
	}

	// Converter para entidades de domínio
	entities := make([]*entity.Entity, 0, len(dbModels))
	for i := range dbModels {
		e, err := dbModels[i].toEntity()
		if err != nil {
			return nil, err
		}
		entities = append(entities, e)
	}

	return &entity.ListResult{
		Entities: entities,
		Total:    total,
		Page:     filter.Page,
		Limit:    filter.Limit,
	}, nil
}

func (r *EntityRepository) Update(ctx context.Context, e *entity.Entity) error {
	query := `
		UPDATE entities SET
			name = :name,
			email = :email,
			active = :active,
			updated_at = :updated_at
		WHERE id = :id
	`

	dbModel := fromDomainEntity(e)
	result, err := r.DB.NamedExecContext(ctx, query, dbModel)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return entity.ErrEntityNotFound
	}

	return nil
}

func (r *EntityRepository) Delete(ctx context.Context, id vo.ID) error {
	query := `
		UPDATE entities SET
			active = false,
			updated_at = $1
		WHERE id = $2 AND active = true
	`

	result, err := r.DB.ExecContext(ctx, query, time.Now(), id.String())
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return entity.ErrEntityNotFound
	}

	return nil
}
