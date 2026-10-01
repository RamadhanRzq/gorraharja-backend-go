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

const paymentColumns = `p.id, p.booking_id, p.amount, p.method, p.status, p.reference,
	p.paid_at, p.verified_by, p.failure_reason, p.created_at, p.updated_at`

// PaymentRepo persists payment attempts.
type PaymentRepo struct {
	db *database.DB
}

// NewPaymentRepo builds a payment repository.
func NewPaymentRepo(db *database.DB) *PaymentRepo { return &PaymentRepo{db: db} }

func scanPayment(row pgx.Row) (*model.Payment, error) {
	var p model.Payment
	err := row.Scan(&p.ID, &p.BookingID, &p.Amount, &p.Method, &p.Status, &p.Reference,
		&p.PaidAt, &p.VerifiedBy, &p.FailureReason, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("payment not found")
		}
		return nil, err
	}
	return &p, nil
}

// PaymentCreateInput is the persisted shape of a new payment.
type PaymentCreateInput struct {
	BookingID     uuid.UUID
	Amount        int64
	Method        string
	Status        model.PaymentStatus
	Reference     *string
	VerifiedBy    *uuid.UUID
	FailureReason string
}

// Create inserts a payment inside tx when provided.
func (r *PaymentRepo) Create(ctx context.Context, tx pgx.Tx, in PaymentCreateInput) (*model.Payment, error) {
	return scanPayment(pick(r.db, tx).QueryRow(ctx, `
		INSERT INTO payments (booking_id, amount, method, status, reference, verified_by, failure_reason, paid_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, CASE WHEN $4 = 'PAID' THEN now() ELSE NULL END)
		RETURNING `+paymentColumns,
		in.BookingID, in.Amount, in.Method, in.Status, in.Reference, in.VerifiedBy, in.FailureReason))
}

// FindByID returns a payment by identifier.
func (r *PaymentRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.Payment, error) {
	return scanPayment(r.db.Pool().QueryRow(ctx,
		`SELECT `+paymentColumns+` FROM payments p WHERE p.id = $1`, id))
}

// FindForBooking returns the newest payment of a booking, or nil when none exists.
func (r *PaymentRepo) FindForBooking(ctx context.Context, bookingID uuid.UUID) (*model.Payment, error) {
	p, err := scanPayment(r.db.Pool().QueryRow(ctx,
		`SELECT `+paymentColumns+` FROM payments p
		 WHERE p.booking_id = $1 ORDER BY p.created_at DESC LIMIT 1`, bookingID))
	if err != nil {
		if apperr.Is(err, apperr.CodeNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return p, nil
}

// PaymentFilter narrows a payment listing.
type PaymentFilter struct {
	BookingID  *uuid.UUID
	CustomerID *uuid.UUID
	Status     *model.PaymentStatus
	Method     *string
	Limit      int
	Offset     int
}

// List returns payments with the owning booking joined for authorization checks.
func (r *PaymentRepo) List(ctx context.Context, f PaymentFilter) ([]model.Payment, int, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT `+paymentColumns+`, count(*) OVER () AS total
		FROM payments p
		JOIN bookings b ON b.id = p.booking_id
		WHERE ($1::uuid IS NULL OR p.booking_id = $1)
		  AND ($2::uuid IS NULL OR b.customer_id = $2)
		  AND ($3::text IS NULL OR p.status = $3)
		  AND ($4::text IS NULL OR p.method = $4)
		ORDER BY p.created_at DESC
		LIMIT $5 OFFSET $6`,
		f.BookingID, f.CustomerID, f.Status, f.Method, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var (
		payments []model.Payment
		total    int
	)
	for rows.Next() {
		var p model.Payment
		if err := rows.Scan(&p.ID, &p.BookingID, &p.Amount, &p.Method, &p.Status, &p.Reference,
			&p.PaidAt, &p.VerifiedBy, &p.FailureReason, &p.CreatedAt, &p.UpdatedAt, &total); err != nil {
			return nil, 0, err
		}
		payments = append(payments, p)
	}
	return payments, total, rows.Err()
}

// SettleInput describes an administrative settlement decision.
type SettleInput struct {
	Status        model.PaymentStatus
	Reference     *string
	VerifiedBy    *uuid.UUID
	FailureReason string
}

// Settle applies an administrative payment decision, guarded by the expected
// source statuses.
func (r *PaymentRepo) Settle(ctx context.Context, tx pgx.Tx, id uuid.UUID, from []model.PaymentStatus, in SettleInput) (*model.Payment, error) {
	if len(from) == 0 {
		return nil, errors.New("payment settlement requires at least one source status")
	}
	return scanPayment(pick(r.db, tx).QueryRow(ctx, `
		UPDATE payments SET
		    status         = $3,
		    reference      = COALESCE($4, reference),
		    verified_by    = COALESCE($5, verified_by),
		    failure_reason = CASE WHEN $6::text = '' THEN failure_reason ELSE $6 END,
		    paid_at        = CASE WHEN $3 = 'PAID' THEN COALESCE(paid_at, now())
		                          WHEN $3 = 'REFUNDED' THEN paid_at
		                          ELSE NULL END
		WHERE id = $1 AND status = ANY($2)
		RETURNING `+paymentColumns,
		id, from, in.Status, in.Reference, in.VerifiedBy, in.FailureReason))
}

// SumPaidForBooking totals settled amounts of a booking, used to prevent
// overpayment when several partial payments exist.
func (r *PaymentRepo) SumPaidForBooking(ctx context.Context, bookingID uuid.UUID) (int64, error) {
	var total *int64
	err := r.db.Pool().QueryRow(ctx, `
		SELECT sum(amount) FROM payments
		WHERE booking_id = $1 AND status = 'PAID'`, bookingID).Scan(&total)
	if err != nil {
		return 0, err
	}
	if total == nil {
		return 0, nil
	}
	return *total, nil
}

// RevenueSummary aggregates settled revenue over a date range, grouped by day.
type RevenueSummary struct {
	Day    model.Date `json:"day"`
	Amount int64      `json:"amount"`
	Count  int        `json:"count"`
}

// RevenueByDay returns settled revenue for bookings dated inside [from, to].
func (r *PaymentRepo) RevenueByDay(ctx context.Context, from, to model.Date) ([]RevenueSummary, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT b.booking_date, COALESCE(sum(p.amount), 0), count(*)
		FROM payments p
		JOIN bookings b ON b.id = p.booking_id
		WHERE p.status = 'PAID' AND b.booking_date BETWEEN $1 AND $2
		GROUP BY b.booking_date
		ORDER BY b.booking_date`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RevenueSummary
	for rows.Next() {
		var s RevenueSummary
		if err := rows.Scan(&s.Day, &s.Amount, &s.Count); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// TouchBookingUpdatedAt is a helper used by tests to simulate clock movement.
func (r *PaymentRepo) TouchBookingUpdatedAt(ctx context.Context, bookingID uuid.UUID, at time.Time) error {
	_, err := r.db.Pool().Exec(ctx, `UPDATE bookings SET updated_at = $2 WHERE id = $1`, bookingID, at)
	return err
}
