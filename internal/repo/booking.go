package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/apperr"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/database"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
)

const bookingColumns = `b.id, b.booking_code, b.customer_id, b.facility_id, b.booking_date,
	b.start_time, b.end_time, b.total_price, b.status, b.notes, b.cancelled_at,
	b.cancelled_by, b.cancel_reason, b.expires_at, b.created_at, b.updated_at`

const bookingFrom = `
	FROM bookings b
	JOIN users u ON u.id = b.customer_id
	JOIN facilities f ON f.id = b.facility_id
	JOIN sports s ON s.id = f.sport_id`

const bookingRelationColumns = `
	u.id, u.email, u.full_name, u.phone, u.role, u.status, u.created_at, u.updated_at,
	f.id, f.sport_id, f.name, f.description, f.location, f.status, f.created_at, f.updated_at,
	s.id, s.name, s.slug, s.description, s.is_active, s.created_at, s.updated_at`

// BookingRepo persists facility reservations.
type BookingRepo struct {
	db *database.DB
}

// NewBookingRepo builds a booking repository.
func NewBookingRepo(db *database.DB) *BookingRepo { return &BookingRepo{db: db} }

func scanBooking(row pgx.Row) (*model.Booking, error) {
	var (
		b model.Booking
		u model.User
		f model.Facility
		s model.Sport
	)
	err := row.Scan(
		&b.ID, &b.BookingCode, &b.CustomerID, &b.FacilityID, &b.BookingDate,
		&b.StartTime, &b.EndTime, &b.TotalPrice, &b.Status, &b.Notes, &b.CancelledAt,
		&b.CancelledBy, &b.CancelReason, &b.ExpiresAt, &b.CreatedAt, &b.UpdatedAt,
		&u.ID, &u.Email, &u.FullName, &u.Phone, &u.Role, &u.Status, &u.CreatedAt, &u.UpdatedAt,
		&f.ID, &f.SportID, &f.Name, &f.Description, &f.Location, &f.Status, &f.CreatedAt, &f.UpdatedAt,
		&s.ID, &s.Name, &s.Slug, &s.Description, &s.IsActive, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("booking not found")
		}
		return nil, err
	}
	f.Sport = &s
	b.Customer = &u
	b.Facility = &f
	return &b, nil
}

// BookingCreateInput is the persisted shape of a new booking.
type BookingCreateInput struct {
	BookingCode string
	CustomerID  uuid.UUID
	FacilityID  uuid.UUID
	Date        model.Date
	StartTime   model.Clock
	EndTime     model.Clock
	TotalPrice  int64
	Notes       string
	ExpiresAt   *time.Time
}

// Create inserts a booking inside tx. The database exclusion constraint on
// overlapping active bookings is the authoritative double-booking guard, so a
// concurrent insert surfaces as a conflict error rather than a duplicate row.
func (r *BookingRepo) Create(ctx context.Context, tx pgx.Tx, in BookingCreateInput) (*model.Booking, error) {
	if tx == nil {
		return nil, fmt.Errorf("booking creation requires a transaction")
	}
	var b model.Booking
	err := tx.QueryRow(ctx, `
		INSERT INTO bookings (booking_code, customer_id, facility_id, booking_date,
		    start_time, end_time, total_price, notes, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, booking_code, customer_id, facility_id, booking_date,
		    start_time, end_time, total_price, status, notes, cancelled_at,
		    cancelled_by, cancel_reason, expires_at, created_at, updated_at`,
		in.BookingCode, in.CustomerID, in.FacilityID, in.Date,
		in.StartTime, in.EndTime, in.TotalPrice, in.Notes, in.ExpiresAt).
		Scan(&b.ID, &b.BookingCode, &b.CustomerID, &b.FacilityID, &b.BookingDate,
			&b.StartTime, &b.EndTime, &b.TotalPrice, &b.Status, &b.Notes, &b.CancelledAt,
			&b.CancelledBy, &b.CancelReason, &b.ExpiresAt, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// FindByID returns a booking with customer, facility and latest payment.
func (r *BookingRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.Booking, error) {
	b, err := scanBooking(r.db.Pool().QueryRow(ctx,
		`SELECT `+bookingColumns+`, `+bookingRelationColumns+` `+bookingFrom+` WHERE b.id = $1`, id))
	if err != nil {
		return nil, err
	}
	payment, err := r.latestPayment(ctx, b.ID)
	if err != nil {
		return nil, err
	}
	b.Payment = payment
	return b, nil
}

// FindByCode returns a booking by its human readable code.
func (r *BookingRepo) FindByCode(ctx context.Context, code string) (*model.Booking, error) {
	b, err := scanBooking(r.db.Pool().QueryRow(ctx,
		`SELECT `+bookingColumns+`, `+bookingRelationColumns+` `+bookingFrom+` WHERE b.booking_code = $1`,
		strings.ToUpper(strings.TrimSpace(code))))
	if err != nil {
		return nil, err
	}
	payment, err := r.latestPayment(ctx, b.ID)
	if err != nil {
		return nil, err
	}
	b.Payment = payment
	return b, nil
}

func (r *BookingRepo) latestPayment(ctx context.Context, bookingID uuid.UUID) (*model.Payment, error) {
	var p model.Payment
	err := r.db.Pool().QueryRow(ctx, `
		SELECT id, booking_id, amount, method, status, reference, paid_at,
		    verified_by, failure_reason, created_at, updated_at
		FROM payments
		WHERE booking_id = $1
		ORDER BY created_at DESC
		LIMIT 1`, bookingID).
		Scan(&p.ID, &p.BookingID, &p.Amount, &p.Method, &p.Status, &p.Reference, &p.PaidAt,
			&p.VerifiedBy, &p.FailureReason, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

// BookingFilter narrows a booking listing.
type BookingFilter struct {
	CustomerID *uuid.UUID
	FacilityID *uuid.UUID
	SportID    *uuid.UUID
	Status     *model.BookingStatus
	Code       string
	DateFrom   *model.Date
	DateTo     *model.Date
	Limit      int
	Offset     int
}

// List returns bookings matching the filter, newest first, with relations and
// the total match count.
func (r *BookingRepo) List(ctx context.Context, f BookingFilter) ([]model.Booking, int, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT `+bookingColumns+`, `+bookingRelationColumns+`, count(*) OVER () AS total
		`+bookingFrom+`
		WHERE ($1::uuid IS NULL OR b.customer_id = $1)
		  AND ($2::uuid IS NULL OR b.facility_id = $2)
		  AND ($3::uuid IS NULL OR f.sport_id = $3)
		  AND ($4::text IS NULL OR b.status = $4)
		  AND ($5::text = '' OR b.booking_code ILIKE '%' || $5 || '%')
		  AND ($6::date IS NULL OR b.booking_date >= $6)
		  AND ($7::date IS NULL OR b.booking_date <= $7)
		ORDER BY b.created_at DESC
		LIMIT $8 OFFSET $9`,
		f.CustomerID, f.FacilityID, f.SportID, f.Status, f.Code, f.DateFrom, f.DateTo, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var (
		bookings []model.Booking
		total    int
	)
	for rows.Next() {
		var (
			b model.Booking
			u model.User
			fac model.Facility
			s model.Sport
		)
		if err := rows.Scan(
			&b.ID, &b.BookingCode, &b.CustomerID, &b.FacilityID, &b.BookingDate,
			&b.StartTime, &b.EndTime, &b.TotalPrice, &b.Status, &b.Notes, &b.CancelledAt,
			&b.CancelledBy, &b.CancelReason, &b.ExpiresAt, &b.CreatedAt, &b.UpdatedAt,
			&u.ID, &u.Email, &u.FullName, &u.Phone, &u.Role, &u.Status, &u.CreatedAt, &u.UpdatedAt,
			&fac.ID, &fac.SportID, &fac.Name, &fac.Description, &fac.Location, &fac.Status,
			&fac.CreatedAt, &fac.UpdatedAt,
			&s.ID, &s.Name, &s.Slug, &s.Description, &s.IsActive, &s.CreatedAt, &s.UpdatedAt,
			&total); err != nil {
			return nil, 0, err
		}
		fac.Sport = &s
		b.Customer = &u
		b.Facility = &fac
		bookings = append(bookings, b)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if err := r.attachPayments(ctx, bookings); err != nil {
		return nil, 0, err
	}
	return bookings, total, nil
}

func (r *BookingRepo) attachPayments(ctx context.Context, bookings []model.Booking) error {
	if len(bookings) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, 0, len(bookings))
	for _, b := range bookings {
		ids = append(ids, b.ID)
	}
	rows, err := r.db.Pool().Query(ctx, `
		SELECT DISTINCT ON (booking_id)
		    id, booking_id, amount, method, status, reference, paid_at,
		    verified_by, failure_reason, created_at, updated_at
		FROM payments
		WHERE booking_id = ANY($1)
		ORDER BY booking_id, created_at DESC`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()

	byBooking := make(map[uuid.UUID]*model.Payment, len(bookings))
	for rows.Next() {
		var p model.Payment
		if err := rows.Scan(&p.ID, &p.BookingID, &p.Amount, &p.Method, &p.Status, &p.Reference,
			&p.PaidAt, &p.VerifiedBy, &p.FailureReason, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return err
		}
		cp := p
		byBooking[p.BookingID] = &cp
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range bookings {
		bookings[i].Payment = byBooking[bookings[i].ID]
	}
	return nil
}

// Slot marks a bookable interval as free or taken, used by availability.
type Slot struct {
	StartTime model.Clock `json:"start_time"`
	EndTime   model.Clock `json:"end_time"`
	Status    string      `json:"status"`
	BookingID string      `json:"booking_id,omitempty"`
}

// BusySlot is a reservation occupying part of a facility day.
type BusySlot struct {
	BookingID   uuid.UUID   `json:"booking_id"`
	BookingCode string      `json:"booking_code"`
	StartTime   model.Clock `json:"start_time"`
	EndTime     model.Clock `json:"end_time"`
	Status      model.BookingStatus `json:"status"`
}

// BusySlots returns every slot reserving capacity on a facility day, so the
// availability service can compute free intervals without a second round trip.
func (r *BookingRepo) BusySlots(ctx context.Context, facilityID uuid.UUID, date model.Date) ([]BusySlot, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT id, booking_code, start_time, end_time, status
		FROM bookings
		WHERE facility_id = $1
		  AND booking_date = $2
		  AND status IN ('PENDING', 'CONFIRMED', 'COMPLETED')
		ORDER BY start_time`, facilityID, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []BusySlot
	for rows.Next() {
		var s BusySlot
		if err := rows.Scan(&s.BookingID, &s.BookingCode, &s.StartTime, &s.EndTime, &s.Status); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// OverlapsBusy reports whether an active booking already occupies any part of
// the requested window. It is a fast pre-check; the exclusion constraint stays
// authoritative for concurrent writes.
func (r *BookingRepo) OverlapsBusy(ctx context.Context, tx pgx.Tx, facilityID uuid.UUID, date model.Date, start, end model.Clock) (bool, error) {
	var exists bool
	err := pick(r.db, tx).QueryRow(ctx, `
		SELECT EXISTS (
		    SELECT 1 FROM bookings
		    WHERE facility_id = $1
		      AND booking_date = $2
		      AND status IN ('PENDING', 'CONFIRMED', 'COMPLETED')
		      AND start_time < $4
		      AND end_time > $3
		)`, facilityID, date, start, end).Scan(&exists)
	return exists, err
}

// Transition updates the status of a booking, guarded by the expected source
// statuses so that concurrent state changes cannot interleave.
func (r *BookingRepo) Transition(ctx context.Context, tx pgx.Tx, id uuid.UUID, from []model.BookingStatus, to model.BookingStatus, patch StatusPatch) (*model.Booking, error) {
	if len(from) == 0 {
		return nil, fmt.Errorf("booking transition requires at least one source status")
	}
	var b model.Booking
	err := pick(r.db, tx).QueryRow(ctx, `
		UPDATE bookings SET
		    status        = $3,
		    cancelled_at  = COALESCE($4, cancelled_at),
		    cancelled_by  = COALESCE($5, cancelled_by),
		    cancel_reason = CASE WHEN $6::text = '' THEN cancel_reason ELSE $6 END,
		    expires_at    = CASE WHEN $7::boolean THEN NULL ELSE expires_at END
		WHERE id = $1 AND status = ANY($2)
		RETURNING id, booking_code, customer_id, facility_id, booking_date,
		    start_time, end_time, total_price, status, notes, cancelled_at,
		    cancelled_by, cancel_reason, expires_at, created_at, updated_at`,
		id, from, to, patch.CancelledAt, patch.CancelledBy, patch.CancelReason, patch.ClearExpiry).
		Scan(&b.ID, &b.BookingCode, &b.CustomerID, &b.FacilityID, &b.BookingDate,
			&b.StartTime, &b.EndTime, &b.TotalPrice, &b.Status, &b.Notes, &b.CancelledAt,
			&b.CancelledBy, &b.CancelReason, &b.ExpiresAt, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.Conflict("the booking status changed in the meantime, please reload")
		}
		return nil, err
	}
	return &b, nil
}

// StatusPatch carries optional side fields applied during a status change.
type StatusPatch struct {
	CancelledAt *time.Time
	CancelledBy *uuid.UUID
	CancelReason string
	ClearExpiry bool
}

// ExpirePending marks stale pending bookings as EXPIRED and returns the count.
// Expired rows drop out of the exclusion constraint, releasing their slot.
func (r *BookingRepo) ExpirePending(ctx context.Context, now time.Time) (int64, error) {
	tag, err := r.db.Pool().Exec(ctx, `
		UPDATE bookings
		SET status = 'EXPIRED'
		WHERE status = 'PENDING'
		  AND expires_at IS NOT NULL
		  AND expires_at <= $1`, now)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// CompleteFinished marks confirmed bookings whose slot has ended as COMPLETED.
func (r *BookingRepo) CompleteFinished(ctx context.Context, now time.Time) (int64, error) {
	tag, err := r.db.Pool().Exec(ctx, `
		UPDATE bookings
		SET status = 'COMPLETED'
		WHERE status = 'CONFIRMED'
		  AND (booking_date + end_time) < $1`, now)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// CountActiveForSlot counts bookings blocking a facility slot. Used by tests
// and administrative projections.
func (r *BookingRepo) CountActiveForSlot(ctx context.Context, facilityID uuid.UUID, date model.Date, start, end model.Clock) (int, error) {
	var n int
	err := r.db.Pool().QueryRow(ctx, `
		SELECT count(*) FROM bookings
		WHERE facility_id = $1 AND booking_date = $2
		  AND status IN ('PENDING', 'CONFIRMED', 'COMPLETED')
		  AND start_time = $3 AND end_time = $4`, facilityID, date, start, end).Scan(&n)
	return n, err
}
