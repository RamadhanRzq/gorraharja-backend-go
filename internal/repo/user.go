// Package repo holds repositories shared by more than one module.
package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/apperr"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/database"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
)

// Querier is satisfied by both *pgxpool.Pool and pgx.Tx.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

const userColumns = `id, email, full_name, phone, password_hash, role, status, created_at, updated_at, deleted_at`

// UserRepo persists user accounts.
type UserRepo struct {
	db *database.DB
}

// NewUserRepo builds a user repository.
func NewUserRepo(db *database.DB) *UserRepo { return &UserRepo{db: db} }

func scanUser(row pgx.Row) (*model.User, error) {
	var u model.User
	err := row.Scan(&u.ID, &u.Email, &u.FullName, &u.Phone, &u.PasswordHash,
		&u.Role, &u.Status, &u.CreatedAt, &u.UpdatedAt, &u.DeletedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("user not found")
		}
		return nil, err
	}
	return &u, nil
}

// FindByID returns a non-deleted user by identifier.
func (r *UserRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	return scanUser(r.db.Pool().QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE id = $1 AND deleted_at IS NULL`, id))
}

// FindByEmail returns a non-deleted user by email address.
func (r *UserRepo) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	return scanUser(r.db.Pool().QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE email = $1 AND deleted_at IS NULL`,
		strings.ToLower(strings.TrimSpace(email))))
}

// Create inserts a new user.
func (r *UserRepo) Create(ctx context.Context, email, fullName, phone, passwordHash string, role model.Role) (*model.User, error) {
	return scanUser(r.db.Pool().QueryRow(ctx,
		`INSERT INTO users (email, full_name, phone, password_hash, role)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING `+userColumns,
		email, fullName, nullIfEmpty(phone), passwordHash, role))
}

// UpdateProfile changes the mutable profile fields of a user.
func (r *UserRepo) UpdateProfile(ctx context.Context, id uuid.UUID, fullName, phone string) (*model.User, error) {
	return scanUser(r.db.Pool().QueryRow(ctx,
		`UPDATE users SET full_name = $2, phone = $3
		 WHERE id = $1 AND deleted_at IS NULL
		 RETURNING `+userColumns,
		id, fullName, nullIfEmpty(phone)))
}

// UpdatePassword replaces the password hash of a user.
func (r *UserRepo) UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	tag, err := r.db.Pool().Exec(ctx,
		`UPDATE users SET password_hash = $2 WHERE id = $1 AND deleted_at IS NULL`, id, passwordHash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("user not found")
	}
	return nil
}

// AdminUpdate changes role and status of a user.
func (r *UserRepo) AdminUpdate(ctx context.Context, id uuid.UUID, role *model.Role, status *model.UserStatus) (*model.User, error) {
	return scanUser(r.db.Pool().QueryRow(ctx,
		`UPDATE users SET
		    role   = COALESCE($2, role),
		    status = COALESCE($3, status)
		 WHERE id = $1 AND deleted_at IS NULL
		 RETURNING `+userColumns,
		id, role, status))
}

// SoftDelete marks a user as deleted.
func (r *UserRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Pool().Exec(ctx,
		`UPDATE users SET deleted_at = now(), status = 'SUSPENDED'
		 WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("user not found")
	}
	return nil
}

// ListFilter narrows an administrative user listing.
type ListFilter struct {
	Role   *model.Role
	Status *model.UserStatus
	Search string
	Limit  int
	Offset int
}

// List returns users matching the filter plus the total match count.
func (r *UserRepo) List(ctx context.Context, f ListFilter) ([]model.User, int, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT `+userColumns+`, count(*) OVER () AS total
		FROM users
		WHERE deleted_at IS NULL
		  AND ($1::text IS NULL OR role = $1)
		  AND ($2::text IS NULL OR status = $2)
		  AND ($3::text = '' OR email ILIKE '%' || $3 || '%' OR full_name ILIKE '%' || $3 || '%')
		ORDER BY created_at DESC
		LIMIT $4 OFFSET $5`,
		f.Role, f.Status, f.Search, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var (
		users []model.User
		total int
	)
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Email, &u.FullName, &u.Phone, &u.PasswordHash,
			&u.Role, &u.Status, &u.CreatedAt, &u.UpdatedAt, &u.DeletedAt, &total); err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	return users, total, rows.Err()
}

// CountAdmins returns how many active admins exist, guarding against lockout.
func (r *UserRepo) CountAdmins(ctx context.Context) (int, error) {
	var n int
	err := r.db.Pool().QueryRow(ctx,
		`SELECT count(*) FROM users WHERE role = 'ADMIN' AND deleted_at IS NULL`).Scan(&n)
	return n, err
}

// EnsureAdmin creates the bootstrap administrator when it does not exist yet.
// It returns true when a new account was created.
func (r *UserRepo) EnsureAdmin(ctx context.Context, email, fullName, passwordHash string) (bool, error) {
	var exists bool
	if err := r.db.Pool().QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM users WHERE email = $1)`,
		strings.ToLower(strings.TrimSpace(email))).Scan(&exists); err != nil {
		return false, fmt.Errorf("check bootstrap admin: %w", err)
	}
	if exists {
		return false, nil
	}
	_, err := r.Create(ctx, email, fullName, "", passwordHash, model.RoleAdmin)
	if err != nil {
		if apperr.Is(err, apperr.CodeConflict) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func nullIfEmpty(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}
