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

const facilityColumns = `f.id, f.sport_id, f.name, f.description, f.location, f.status,
	f.created_at, f.updated_at,
	s.id, s.name, s.slug, s.description, s.is_active, s.created_at, s.updated_at`

const facilityFrom = `FROM facilities f JOIN sports s ON s.id = f.sport_id`

func scanFacility(row pgx.Row) (*model.Facility, error) {
	var (
		f model.Facility
		s model.Sport
	)
	err := row.Scan(
		&f.ID, &f.SportID, &f.Name, &f.Description, &f.Location, &f.Status,
		&f.CreatedAt, &f.UpdatedAt,
		&s.ID, &s.Name, &s.Slug, &s.Description, &s.IsActive, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("facility not found")
		}
		return nil, err
	}
	f.Sport = &s
	return &f, nil
}

// FacilityRepo persists bookable facilities.
type FacilityRepo struct {
	db *database.DB
}

// NewFacilityRepo builds a facility repository.
func NewFacilityRepo(db *database.DB) *FacilityRepo { return &FacilityRepo{db: db} }

// FacilityFilter narrows a facility listing.
type FacilityFilter struct {
	SportID    *uuid.UUID
	Status     *model.FacilityStatus
	ActiveOnly bool
	Search     string
	Limit      int
	Offset     int
}

// List returns facilities with their sport expanded plus the total match count.
func (r *FacilityRepo) List(ctx context.Context, f FacilityFilter) ([]model.Facility, int, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT `+facilityColumns+`, count(*) OVER () AS total
		`+facilityFrom+`
		WHERE ($1::uuid IS NULL OR f.sport_id = $1)
		  AND ($2::text IS NULL OR f.status = $2)
		  AND ($3::boolean IS FALSE OR (f.status = 'ACTIVE' AND s.is_active))
		  AND ($4::text = '' OR f.name ILIKE '%' || $4 || '%' OR f.location ILIKE '%' || $4 || '%')
		ORDER BY s.name, f.name
		LIMIT $5 OFFSET $6`,
		f.SportID, f.Status, f.ActiveOnly, f.Search, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var (
		facilities []model.Facility
		total      int
	)
	for rows.Next() {
		var (
			fac model.Facility
			s   model.Sport
		)
		if err := rows.Scan(
			&fac.ID, &fac.SportID, &fac.Name, &fac.Description, &fac.Location, &fac.Status,
			&fac.CreatedAt, &fac.UpdatedAt,
			&s.ID, &s.Name, &s.Slug, &s.Description, &s.IsActive, &s.CreatedAt, &s.UpdatedAt,
			&total); err != nil {
			return nil, 0, err
		}
		fac.Sport = &s
		facilities = append(facilities, fac)
	}
	return facilities, total, rows.Err()
}

// FindByID returns a facility with its sport expanded.
func (r *FacilityRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.Facility, error) {
	return scanFacility(r.db.Pool().QueryRow(ctx,
		`SELECT `+facilityColumns+` `+facilityFrom+` WHERE f.id = $1`, id))
}

// Create inserts a new facility.
func (r *FacilityRepo) Create(ctx context.Context, sportID uuid.UUID, name, description, location string, status model.FacilityStatus) (*model.Facility, error) {
	var id uuid.UUID
	if err := r.db.Pool().QueryRow(ctx, `
		INSERT INTO facilities (sport_id, name, description, location, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`,
		sportID, name, description, location, status).Scan(&id); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

// FacilityUpdate carries the mutable fields of a facility; nil fields stay unchanged.
type FacilityUpdate struct {
	SportID     *uuid.UUID
	Name        *string
	Description *string
	Location    *string
	Status      *model.FacilityStatus
}

// Update applies a partial update to a facility.
func (r *FacilityRepo) Update(ctx context.Context, id uuid.UUID, in FacilityUpdate) (*model.Facility, error) {
	var updated uuid.UUID
	err := r.db.Pool().QueryRow(ctx, `
		UPDATE facilities SET
		    sport_id    = COALESCE($2, sport_id),
		    name        = COALESCE($3, name),
		    description = COALESCE($4, description),
		    location    = COALESCE($5, location),
		    status      = COALESCE($6, status)
		WHERE id = $1
		RETURNING id`,
		id, in.SportID, in.Name, in.Description, in.Location, in.Status).Scan(&updated)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("facility not found")
		}
		return nil, err
	}
	return r.FindByID(ctx, updated)
}

// Delete removes a facility. Facilities with booking history are reported as a
// conflict so that reservations are never silently orphaned.
func (r *FacilityRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Pool().Exec(ctx, `DELETE FROM facilities WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("facility not found")
	}
	return nil
}
