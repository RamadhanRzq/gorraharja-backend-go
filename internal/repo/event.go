package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/apperr"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/database"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
)

const eventColumns = `e.id, e.title, e.description, e.event_type, e.start_at, e.end_at,
	e.location, e.capacity, e.status, e.registration_fee, e.created_by,
	e.created_at, e.updated_at,
	COALESCE(r.registered, 0) AS registered_count`

// registeredSub counts active registrations per event.
const registeredSub = `
	LEFT JOIN (
	    SELECT event_id, count(*) AS registered
	    FROM event_registrations
	    WHERE status = 'REGISTERED'
	    GROUP BY event_id
	) r ON r.event_id = e.id`

// EventRepo persists events and their registrations.
type EventRepo struct {
	db *database.DB
}

// NewEventRepo builds an event repository.
func NewEventRepo(db *database.DB) *EventRepo { return &EventRepo{db: db} }

func scanEvent(row pgx.Row) (*model.Event, error) {
	var e model.Event
	err := row.Scan(&e.ID, &e.Title, &e.Description, &e.EventType, &e.StartAt, &e.EndAt,
		&e.Location, &e.Capacity, &e.Status, &e.RegistrationFee, &e.CreatedBy,
		&e.CreatedAt, &e.UpdatedAt, &e.RegisteredCount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("event not found")
		}
		return nil, err
	}
	e.SeatsRemaining = e.Capacity - e.RegisteredCount
	return &e, nil
}

// EventFilter narrows an event listing.
type EventFilter struct {
	Status       *model.EventStatus
	PublishedOnly bool
	UpcomingOnly bool
	Search       string
	Limit        int
	Offset       int
}

// List returns events with registration counts plus the total match count.
func (r *EventRepo) List(ctx context.Context, f EventFilter) ([]model.Event, int, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT `+eventColumns+`, count(*) OVER () AS total
		FROM events e
		`+registeredSub+`
		WHERE ($1::text IS NULL OR e.status = $1)
		  AND ($2::boolean IS FALSE OR e.status IN ('PUBLISHED', 'ONGOING'))
		  AND ($3::boolean IS FALSE OR e.end_at >= now())
		  AND ($4::text = '' OR e.title ILIKE '%' || $4 || '%' OR e.location ILIKE '%' || $4 || '%')
		ORDER BY e.start_at DESC
		LIMIT $5 OFFSET $6`,
		f.Status, f.PublishedOnly, f.UpcomingOnly, f.Search, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var (
		events []model.Event
		total  int
	)
	for rows.Next() {
		var e model.Event
		if err := rows.Scan(&e.ID, &e.Title, &e.Description, &e.EventType, &e.StartAt, &e.EndAt,
			&e.Location, &e.Capacity, &e.Status, &e.RegistrationFee, &e.CreatedBy,
			&e.CreatedAt, &e.UpdatedAt, &e.RegisteredCount, &total); err != nil {
			return nil, 0, err
		}
		e.SeatsRemaining = e.Capacity - e.RegisteredCount
		events = append(events, e)
	}
	return events, total, rows.Err()
}

// FindByID returns an event by identifier.
func (r *EventRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.Event, error) {
	return scanEvent(r.db.Pool().QueryRow(ctx,
		`SELECT `+eventColumns+` FROM events e `+registeredSub+` WHERE e.id = $1`, id))
}

// EventCreateInput is the persisted shape of a new event.
type EventCreateInput struct {
	Title           string
	Description     string
	EventType       string
	StartAt         time.Time
	EndAt           time.Time
	Location        string
	Capacity        int
	Status          model.EventStatus
	RegistrationFee int64
	CreatedBy       *uuid.UUID
}

// Create inserts an event.
func (r *EventRepo) Create(ctx context.Context, in EventCreateInput) (*model.Event, error) {
	var id uuid.UUID
	err := r.db.Pool().QueryRow(ctx, `
		INSERT INTO events (title, description, event_type, start_at, end_at, location,
		    capacity, status, registration_fee, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id`,
		in.Title, in.Description, in.EventType, in.StartAt, in.EndAt, in.Location,
		in.Capacity, in.Status, in.RegistrationFee, in.CreatedBy).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

// EventUpdate carries the mutable fields of an event; nil fields stay unchanged.
type EventUpdate struct {
	Title           *string
	Description     *string
	EventType       *string
	StartAt         *time.Time
	EndAt           *time.Time
	Location        *string
	Capacity        *int
	Status          *model.EventStatus
	RegistrationFee *int64
}

// Update applies a partial update to an event.
func (r *EventRepo) Update(ctx context.Context, id uuid.UUID, in EventUpdate) (*model.Event, error) {
	var updated uuid.UUID
	err := r.db.Pool().QueryRow(ctx, `
		UPDATE events SET
		    title            = COALESCE($2, title),
		    description      = COALESCE($3, description),
		    event_type       = COALESCE($4, event_type),
		    start_at         = COALESCE($5, start_at),
		    end_at           = COALESCE($6, end_at),
		    location         = COALESCE($7, location),
		    capacity         = COALESCE($8, capacity),
		    status           = COALESCE($9, status),
		    registration_fee = COALESCE($10, registration_fee)
		WHERE id = $1
		RETURNING id`,
		id, in.Title, in.Description, in.EventType, in.StartAt, in.EndAt, in.Location,
		in.Capacity, in.Status, in.RegistrationFee).Scan(&updated)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("event not found")
		}
		return nil, err
	}
	return r.FindByID(ctx, updated)
}

// Delete removes an event together with its registrations.
func (r *EventRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Pool().Exec(ctx, `DELETE FROM events WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("event not found")
	}
	return nil
}

// LockEventRow takes a row lock so capacity checks and inserts serialize.
func (r *EventRepo) LockEventRow(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*model.Event, error) {
	var (
		status   model.EventStatus
		capacity int
	)
	err := tx.QueryRow(ctx,
		`SELECT status, capacity FROM events WHERE id = $1 FOR UPDATE`, id).
		Scan(&status, &capacity)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("event not found")
		}
		return nil, err
	}
	return &model.Event{ID: id, Status: status, Capacity: capacity}, nil
}

// CountRegistrations counts active registrations of an event.
func (r *EventRepo) CountRegistrations(ctx context.Context, tx pgx.Tx, eventID uuid.UUID) (int, error) {
	var n int
	err := pick(r.db, tx).QueryRow(ctx,
		`SELECT count(*) FROM event_registrations WHERE event_id = $1 AND status = 'REGISTERED'`,
		eventID).Scan(&n)
	return n, err
}

// Register inserts an event registration inside tx.
func (r *EventRepo) Register(ctx context.Context, tx pgx.Tx, eventID, customerID uuid.UUID, notes string) (*model.EventRegistration, error) {
	var reg model.EventRegistration
	err := tx.QueryRow(ctx, `
		INSERT INTO event_registrations (event_id, customer_id, notes)
		VALUES ($1, $2, $3)
		RETURNING id, event_id, customer_id, status, notes, created_at, updated_at, cancelled_at`,
		eventID, customerID, notes).
		Scan(&reg.ID, &reg.EventID, &reg.CustomerID, &reg.Status, &reg.Notes,
			&reg.CreatedAt, &reg.UpdatedAt, &reg.CancelledAt)
	if err != nil {
		return nil, err
	}
	return &reg, nil
}

// FindRegistration returns a customer's registration for an event.
func (r *EventRepo) FindRegistration(ctx context.Context, eventID, customerID uuid.UUID) (*model.EventRegistration, error) {
	var reg model.EventRegistration
	err := r.db.Pool().QueryRow(ctx, `
		SELECT id, event_id, customer_id, status, notes, created_at, updated_at, cancelled_at
		FROM event_registrations
		WHERE event_id = $1 AND customer_id = $2`, eventID, customerID).
		Scan(&reg.ID, &reg.EventID, &reg.CustomerID, &reg.Status, &reg.Notes,
			&reg.CreatedAt, &reg.UpdatedAt, &reg.CancelledAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("registration not found")
		}
		return nil, err
	}
	return &reg, nil
}

// CancelRegistration revokes a customer's active registration.
func (r *EventRepo) CancelRegistration(ctx context.Context, tx pgx.Tx, eventID, customerID uuid.UUID) error {
	tag, err := pick(r.db, tx).Exec(ctx, `
		UPDATE event_registrations
		SET status = 'CANCELLED', cancelled_at = now()
		WHERE event_id = $1 AND customer_id = $2 AND status = 'REGISTERED'`, eventID, customerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("no active registration found")
	}
	return nil
}

// RegistrationFilter narrows a registration listing.
type RegistrationFilter struct {
	EventID uuid.UUID
	Status  *model.RegistrationStatus
	Limit   int
	Offset  int
}

// ListRegistrations returns registrations of an event with the customer expanded.
func (r *EventRepo) ListRegistrations(ctx context.Context, f RegistrationFilter) ([]model.EventRegistration, int, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT er.id, er.event_id, er.customer_id, er.status, er.notes,
		    er.created_at, er.updated_at, er.cancelled_at,
		    u.id, u.email, u.full_name, u.phone, u.role, u.status, u.created_at, u.updated_at,
		    count(*) OVER () AS total
		FROM event_registrations er
		JOIN users u ON u.id = er.customer_id
		WHERE er.event_id = $1
		  AND ($2::text IS NULL OR er.status = $2)
		ORDER BY er.created_at
		LIMIT $3 OFFSET $4`,
		f.EventID, f.Status, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var (
		regs  []model.EventRegistration
		total int
	)
	for rows.Next() {
		var (
			reg model.EventRegistration
			u   model.User
		)
		if err := rows.Scan(&reg.ID, &reg.EventID, &reg.CustomerID, &reg.Status, &reg.Notes,
			&reg.CreatedAt, &reg.UpdatedAt, &reg.CancelledAt,
			&u.ID, &u.Email, &u.FullName, &u.Phone, &u.Role, &u.Status, &u.CreatedAt, &u.UpdatedAt,
			&total); err != nil {
			return nil, 0, err
		}
		reg.Customer = &u
		regs = append(regs, reg)
	}
	return regs, total, rows.Err()
}

// ListCustomerRegistrations returns the events a customer registered for.
func (r *EventRepo) ListCustomerRegistrations(ctx context.Context, customerID uuid.UUID, limit, offset int) ([]model.EventRegistration, int, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT er.id, er.event_id, er.customer_id, er.status, er.notes,
		    er.created_at, er.updated_at, er.cancelled_at, count(*) OVER () AS total
		FROM event_registrations er
		WHERE er.customer_id = $1
		ORDER BY er.created_at DESC
		LIMIT $2 OFFSET $3`, customerID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var (
		regs  []model.EventRegistration
		total int
	)
	for rows.Next() {
		var reg model.EventRegistration
		if err := rows.Scan(&reg.ID, &reg.EventID, &reg.CustomerID, &reg.Status, &reg.Notes,
			&reg.CreatedAt, &reg.UpdatedAt, &reg.CancelledAt, &total); err != nil {
			return nil, 0, err
		}
		regs = append(regs, reg)
	}
	return regs, total, rows.Err()
}
