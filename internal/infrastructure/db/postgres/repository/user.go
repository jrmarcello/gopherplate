package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	userdomain "github.com/jrmarcello/go-boilerplate/internal/domain/user"
	"github.com/jrmarcello/go-boilerplate/internal/domain/user/vo"
)

// userDB é o modelo de banco de dados (Data Model).
//
// Por que ter um modelo separado da entidade de domínio?
//   - Nomes de colunas podem ser diferentes dos campos da entidade
//   - Permite adicionar campos específicos do banco (audit, soft delete)
//   - Desacopla mudanças de schema de mudanças de domínio
type userDB struct {
	ID        string    `db:"id"`
	Name      string    `db:"name"`
	Email     string    `db:"email"`
	Active    bool      `db:"active"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (e *userDB) toUser() (*userdomain.User, error) {
	id, parseErr := vo.ParseID(e.ID)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing ID: %w", parseErr)
	}

	email, emailErr := vo.NewEmail(e.Email)
	if emailErr != nil {
		return nil, fmt.Errorf("parsing email: %w", emailErr)
	}

	return &userdomain.User{
		ID:        id,
		Name:      e.Name,
		Email:     email,
		Active:    e.Active,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
	}, nil
}

func fromDomainUser(e *userdomain.User) userDB {
	return userDB{
		ID:        e.ID.String(),
		Name:      e.Name,
		Email:     e.Email.String(),
		Active:    e.Active,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
	}
}

// UserRepository implementa a interface Repository para User.
type UserRepository struct {
	writer *sqlx.DB
	reader *sqlx.DB
}

// NewUserRepository cria uma nova instância do repositório.
func NewUserRepository(writer, reader *sqlx.DB) *UserRepository {
	return &UserRepository{writer: writer, reader: reader}
}

func (r *UserRepository) Create(ctx context.Context, e *userdomain.User) error {
	query := `
		INSERT INTO users (
			id, name, email, active, created_at, updated_at
		) VALUES (
			:id, :name, :email, :active, :created_at, :updated_at
		)
	`

	dbModel := fromDomainUser(e)
	_, execErr := r.writer.NamedExecContext(ctx, query, dbModel)
	return execErr
}

func (r *UserRepository) FindByID(ctx context.Context, id vo.ID) (*userdomain.User, error) {
	query := `
		SELECT id, name, email, active, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	var dbModel userDB
	selectErr := r.reader.GetContext(ctx, &dbModel, query, id.String())
	if selectErr != nil {
		if errors.Is(selectErr, sql.ErrNoRows) {
			return nil, userdomain.ErrUserNotFound
		}
		return nil, selectErr
	}

	return dbModel.toUser()
}

func (r *UserRepository) FindByEmail(ctx context.Context, email vo.Email) (*userdomain.User, error) {
	query := `
		SELECT id, name, email, active, created_at, updated_at
		FROM users
		WHERE email = $1
	`

	var dbModel userDB
	selectErr := r.reader.GetContext(ctx, &dbModel, query, email.String())
	if selectErr != nil {
		if errors.Is(selectErr, sql.ErrNoRows) {
			return nil, userdomain.ErrUserNotFound
		}
		return nil, selectErr
	}

	return dbModel.toUser()
}

func (r *UserRepository) List(ctx context.Context, filter userdomain.ListFilter) (*userdomain.ListResult, error) {
	filter.Normalize()

	// Build dynamic query with filters
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

	// Wrap COUNT + SELECT in a read-only transaction for consistent pagination.
	// Without a transaction, rows could be inserted/deleted between the two queries,
	// causing total count to be inconsistent with the returned data.
	tx, txErr := r.reader.BeginTxx(ctx, &sql.TxOptions{ReadOnly: true})
	if txErr != nil {
		return nil, fmt.Errorf("beginning read transaction: %w", txErr)
	}
	defer func() { _ = tx.Rollback() }()

	// Count query
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM users %s", whereClause)
	var total int

	countQuery, countArgs, namedErr := sqlx.Named(countQuery, args)
	if namedErr != nil {
		return nil, namedErr
	}
	countQuery = tx.Rebind(countQuery)

	countErr := tx.GetContext(ctx, &total, countQuery, countArgs...)
	if countErr != nil {
		return nil, countErr
	}

	// Paginated data query
	args["limit"] = filter.Limit
	args["offset"] = filter.Offset()

	dataQuery := fmt.Sprintf(`
		SELECT id, name, email, active, created_at, updated_at
		FROM users
		%s
		ORDER BY created_at DESC
		LIMIT :limit OFFSET :offset
	`, whereClause)

	dataQuery, dataArgs, dataNamedErr := sqlx.Named(dataQuery, args)
	if dataNamedErr != nil {
		return nil, dataNamedErr
	}
	dataQuery = tx.Rebind(dataQuery)

	var dbModels []userDB
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

	// Convert to domain users
	users := make([]*userdomain.User, 0, len(dbModels))
	for i := range dbModels {
		u, convertErr := dbModels[i].toUser()
		if convertErr != nil {
			return nil, convertErr
		}
		users = append(users, u)
	}

	return &userdomain.ListResult{
		Users: users,
		Total: total,
		Page:  filter.Page,
		Limit: filter.Limit,
	}, nil
}

func (r *UserRepository) Update(ctx context.Context, e *userdomain.User) error {
	tx, txErr := r.writer.BeginTxx(ctx, nil)
	if txErr != nil {
		return fmt.Errorf("beginning transaction: %w", txErr)
	}
	defer func() { _ = tx.Rollback() }()

	query := `
		UPDATE users SET
			name = :name,
			email = :email,
			active = :active,
			updated_at = :updated_at
		WHERE id = :id
	`

	dbModel := fromDomainUser(e)
	result, execErr := tx.NamedExecContext(ctx, query, dbModel)
	if execErr != nil {
		return execErr
	}

	rowsAffected, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return rowsErr
	}

	if rowsAffected == 0 {
		return userdomain.ErrUserNotFound
	}

	commitErr := tx.Commit()
	if commitErr != nil {
		return fmt.Errorf("committing transaction: %w", commitErr)
	}

	return nil
}

func (r *UserRepository) Delete(ctx context.Context, id vo.ID) error {
	query := `
		UPDATE users SET
			active = false,
			updated_at = $1
		WHERE id = $2 AND active = true
	`

	result, execErr := r.writer.ExecContext(ctx, query, time.Now(), id.String())
	if execErr != nil {
		return execErr
	}

	rowsAffected, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return rowsErr
	}

	if rowsAffected == 0 {
		return userdomain.ErrUserNotFound
	}

	return nil
}
