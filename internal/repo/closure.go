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

const closureColumns = `id, facility_id, start_date, end_date, start_time, end_time, reason, created_by, created_at, updated_at`

// ClosureRepo persists facility closures (maintenance windows).
type ClosureRepo struct {
	db *database.DB
}

// NewClosureRepo builds a closure repository.
func NewClosureRepo(db *database.DB) *ClosureRepo { return &ClosureRepo{db: db} }

func scanClosure(row pgx.Row) (*model.FacilityClosure, error) {
	var c model.FacilityClosure
	err := row.Scan(&c.ID, &c.FacilityID, &c.StartDate, &c.EndDate, &c.StartTime, &c.EndTime,
		&c.Reason, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("facility closure not found")
		}
		return nil, err
	}
	return &c, nil
}

// ListByFacility returns closures of a facility, optionally limited to those
// overlapping a date range.
func (r *ClosureRepo) ListByFacility(ctx context.Context, facilityID uuid.UUID, from, to *model.Date) ([]model.FacilityClosure, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT `+closureColumns+`
		FROM facility_closures
		WHERE facility_id = $1
		  AND ($2::date IS NULL OR end_date >= $2)
		  AND ($3::date IS NULL OR start_date <= $3)
		ORDER BY start_date, start_time NULLS FIRST`,
		facilityID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.FacilityClosure
	for rows.Next() {
		var c model.FacilityClosure
		if err := rows.Scan(&c.ID, &c.FacilityID, &c.StartDate, &c.EndDate, &c.StartTime, &c.EndTime,
			&c.Reason, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// FindByID returns a facility closure by identifier.
func (r *ClosureRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.FacilityClosure, error) {
	return scanClosure(r.db.Pool().QueryRow(ctx,
		`SELECT `+closureColumns+` FROM facility_closures WHERE id = $1`, id))
}

// Create inserts a facility closure.
func (r *ClosureRepo) Create(ctx context.Context, in model.FacilityClosure) (*model.FacilityClosure, error) {
	return scanClosure(r.db.Pool().QueryRow(ctx, `
		INSERT INTO facility_closures (facility_id, start_date, end_date, start_time, end_time, reason, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+closureColumns,
		in.FacilityID, in.StartDate, in.EndDate, in.StartTime, in.EndTime, in.Reason, in.CreatedBy))
}

// Delete removes a facility closure.
func (r *ClosureRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Pool().Exec(ctx, `DELETE FROM facility_closures WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("facility closure not found")
	}
	return nil
}
