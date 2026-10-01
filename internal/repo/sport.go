package repo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/apperr"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/database"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
)

const sportColumns = `id, name, slug, description, is_active, created_at, updated_at`

// SportRepo persists configurable sport types.
type SportRepo struct {
	db *database.DB
}

// NewSportRepo builds a sport repository.
func NewSportRepo(db *database.DB) *SportRepo { return &SportRepo{db: db} }

func scanSport(row pgx.Row) (*model.Sport, error) {
	var s model.Sport
	err := row.Scan(&s.ID, &s.Name, &s.Slug, &s.Description, &s.IsActive, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("sport not found")
		}
		return nil, err
	}
	return &s, nil
}

// SportFilter narrows a sport listing.
type SportFilter struct {
	ActiveOnly bool
	Search     string
	Limit      int
	Offset     int
}

// List returns sports matching the filter plus the total match count.
func (r *SportRepo) List(ctx context.Context, f SportFilter) ([]model.Sport, int, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT `+sportColumns+`, count(*) OVER () AS total
		FROM sports
		WHERE ($1::boolean IS FALSE OR is_active)
		  AND ($2::text = '' OR name ILIKE '%' || $2 || '%' OR slug ILIKE '%' || $2 || '%')
		ORDER BY name
		LIMIT $3 OFFSET $4`,
		f.ActiveOnly, f.Search, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var (
		sports []model.Sport
		total  int
	)
	for rows.Next() {
		var s model.Sport
		if err := rows.Scan(&s.ID, &s.Name, &s.Slug, &s.Description, &s.IsActive,
			&s.CreatedAt, &s.UpdatedAt, &total); err != nil {
			return nil, 0, err
		}
		sports = append(sports, s)
	}
	return sports, total, rows.Err()
}

// FindByID returns a sport by identifier.
func (r *SportRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.Sport, error) {
	return scanSport(r.db.Pool().QueryRow(ctx,
		`SELECT `+sportColumns+` FROM sports WHERE id = $1`, id))
}

// Create inserts a new sport.
func (r *SportRepo) Create(ctx context.Context, name, slug, description string, isActive bool) (*model.Sport, error) {
	return scanSport(r.db.Pool().QueryRow(ctx,
		`INSERT INTO sports (name, slug, description, is_active)
		 VALUES ($1, $2, $3, $4)
		 RETURNING `+sportColumns,
		name, slug, description, isActive))
}

// SportUpdate carries the mutable fields of a sport; nil fields stay unchanged.
type SportUpdate struct {
	Name        *string
	Slug        *string
	Description *string
	IsActive    *bool
}

// Update applies a partial update to a sport.
func (r *SportRepo) Update(ctx context.Context, id uuid.UUID, in SportUpdate) (*model.Sport, error) {
	return scanSport(r.db.Pool().QueryRow(ctx, `
		UPDATE sports SET
		    name        = COALESCE($2, name),
		    slug        = COALESCE($3, slug),
		    description = COALESCE($4, description),
		    is_active   = COALESCE($5, is_active)
		WHERE id = $1
		RETURNING `+sportColumns,
		id, in.Name, in.Slug, in.Description, in.IsActive))
}

// Delete removes a sport. Sports still referenced by facilities are reported as
// a conflict instead of silently cascading.
func (r *SportRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Pool().Exec(ctx, `DELETE FROM sports WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("sport not found")
	}
	return nil
}
