package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	roledomain "github.com/jrmarcello/go-boilerplate/internal/domain/role"
	"github.com/jrmarcello/go-boilerplate/internal/domain/user/vo"
)

// roleDB é o modelo de banco de dados (Data Model) para Role.
//
// Por que ter um modelo separado da entidade de domínio?
//   - Nomes de colunas podem ser diferentes dos campos da entidade
//   - Permite adicionar campos específicos do banco (audit, soft delete)
//   - Desacopla mudanças de schema de mudanças de domínio
type roleDB struct {
	ID          string    `db:"id"`
	Name        string    `db:"name"`
	Description string    `db:"description"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

func (r *roleDB) toRole() (*roledomain.Role, error) {
	id, parseErr := vo.ParseID(r.ID)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing ID: %w", parseErr)
	}

	return &roledomain.Role{
		ID:          id,
		Name:        r.Name,
		Description: r.Description,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}, nil
}

func fromDomainRole(r *roledomain.Role) roleDB {
	return roleDB{
		ID:          r.ID.String(),
		Name:        r.Name,
		Description: r.Description,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

// RoleRepository implementa a interface Repository para Role.
type RoleRepository struct {
	writer *sqlx.DB
	reader *sqlx.DB
}

// NewRoleRepository cria uma nova instância do repositório.
func NewRoleRepository(writer, reader *sqlx.DB) *RoleRepository {
	return &RoleRepository{writer: writer, reader: reader}
}

func (r *RoleRepository) Create(ctx context.Context, role *roledomain.Role) error {
	query := `
		INSERT INTO roles (
			id, name, description, created_at, updated_at
		) VALUES (
			:id, :name, :description, :created_at, :updated_at
		)
	`

	dbModel := fromDomainRole(role)
	_, execErr := r.writer.NamedExecContext(ctx, query, dbModel)
	return execErr
}

func (r *RoleRepository) FindByName(ctx context.Context, name string) (*roledomain.Role, error) {
	query := `
		SELECT id, name, description, created_at, updated_at
		FROM roles
		WHERE name = $1
	`

	var dbModel roleDB
	selectErr := r.reader.GetContext(ctx, &dbModel, query, name)
	if selectErr != nil {
		if errors.Is(selectErr, sql.ErrNoRows) {
			return nil, roledomain.ErrRoleNotFound
		}
		return nil, selectErr
	}

	return dbModel.toRole()
}

func (r *RoleRepository) List(ctx context.Context, filter roledomain.ListFilter) (*roledomain.ListResult, error) {
	filter.Normalize()

	// Build dynamic query with filters
	args := make(map[string]interface{})

	whereClause := ""
	if filter.Name != "" {
		whereClause = "WHERE name ILIKE :name"
		args["name"] = "%" + filter.Name + "%"
	}

	// Wrap COUNT + SELECT in a read-only transaction for consistent pagination.
	// Without a transaction, rows could be inserted/deleted between the two queries,
	// causing total count to be inconsistent with the returned data.
	tx, txErr := r.reader.BeginTxx(ctx, &sql.TxOptions{ReadOnly: true})
	if txErr != nil {
		return nil, fmt.Errorf("beginning read transaction: %w", txErr)
	}
	defer func() { _ = tx.Rollback() }()

	// Count query
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM roles %s", whereClause)

	countQuery, countArgs, namedErr := sqlx.Named(countQuery, args)
	if namedErr != nil {
		return nil, namedErr
	}
	countQuery = tx.Rebind(countQuery)

	var total int
	countErr := tx.GetContext(ctx, &total, countQuery, countArgs...)
	if countErr != nil {
		return nil, countErr
	}

	// Paginated data query
	args["limit"] = filter.Limit
	args["offset"] = filter.Offset()

	dataQuery := fmt.Sprintf(`
		SELECT id, name, description, created_at, updated_at
		FROM roles
		%s
		ORDER BY created_at DESC
		LIMIT :limit OFFSET :offset
	`, whereClause)

	dataQuery, dataArgs, dataNamedErr := sqlx.Named(dataQuery, args)
	if dataNamedErr != nil {
		return nil, dataNamedErr
	}
	dataQuery = tx.Rebind(dataQuery)

	var dbModels []roleDB
	selectErr := tx.SelectContext(ctx, &dbModels, dataQuery, dataArgs...)
	if selectErr != nil {
		return nil, selectErr
	}

	// Commit the read-only transaction (also valid to let defer Rollback handle it,
	// but explicit commit is cleaner for read-only transactions).
	commitErr := tx.Commit()
	if commitErr != nil {
		return nil, fmt.Errorf("committing read transaction: %w", commitErr)
	}

	// Convert to domain roles
	roles := make([]*roledomain.Role, 0, len(dbModels))
	for i := range dbModels {
		role, convertErr := dbModels[i].toRole()
		if convertErr != nil {
			return nil, convertErr
		}
		roles = append(roles, role)
	}

	return &roledomain.ListResult{
		Roles: roles,
		Total: total,
		Page:  filter.Page,
		Limit: filter.Limit,
	}, nil
}

func (r *RoleRepository) Delete(ctx context.Context, id vo.ID) error {
	query := `
		DELETE FROM roles
		WHERE id = $1
	`

	result, execErr := r.writer.ExecContext(ctx, query, id.String())
	if execErr != nil {
		return execErr
	}

	rowsAffected, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return rowsErr
	}

	if rowsAffected == 0 {
		return roledomain.ErrRoleNotFound
	}

	return nil
}
