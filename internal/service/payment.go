package service

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/apperr"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/audit"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/database"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/repo"
)

// PaymentService records payment attempts and settles them administratively.
// The amount always comes from the booking, so a client can never underpay by
// declaring its own total.
type PaymentService struct {
	db       *database.DB
	audit    *audit.Recorder
	payments *repo.PaymentRepo
	bookings *repo.BookingRepo
}

// PaymentInput is the create payload of a payment attempt.
type PaymentInput struct {
	BookingID uuid.UUID `json:"booking_id" validate:"required"`
	Method    string    `json:"method" validate:"required,oneof=CASH BANK_TRANSFER QRIS EWALLET OTHER"`
	Reference string    `json:"reference" validate:"omitempty,max=120"`
}

// PaymentSettleInput is the administrative decision payload.
type PaymentSettleInput struct {
	Status        model.PaymentStatus `json:"status" validate:"required"`
	Reference     string              `json:"reference" validate:"omitempty,max=120"`
	FailureReason string              `json:"failure_reason" validate:"omitempty,max=500"`
}

// Create records a payment for a booking. A staff member taking cash settles it
// immediately; every other method stays pending until an administrator
// verifies it.
func (s *PaymentService) Create(ctx context.Context, actor *model.User, in PaymentInput, meta RequestMeta) (*model.Payment, error) {
	if err := requireActor(actor); err != nil {
		return nil, err
	}
	booking, err := s.bookings.FindByID(ctx, in.BookingID)
	if err != nil {
		return nil, err
	}
	if err := requireOwnerOrStaff(actor, booking.CustomerID); err != nil {
		return nil, err
	}
	if booking.Status != model.BookingPending && booking.Status != model.BookingConfirmed {
		return nil, apperr.Unprocessable("booking is not awaiting payment")
	}

	method := strings.ToUpper(strings.TrimSpace(in.Method))
	status := model.PaymentPending
	var verifiedBy *uuid.UUID
	if actor.Role.IsStaffOrAdmin() && method == "CASH" {
		status = model.PaymentPaid
		staff := actor.ID
		verifiedBy = &staff
	}
	reference := strings.TrimSpace(in.Reference)
	var ref *string
	if reference != "" {
		ref = &reference
	}

	var payment *model.Payment
	err = s.db.InTx(ctx, func(tx pgx.Tx) error {
		paid, err := s.payments.SumPaidForBooking(ctx, booking.ID)
		if err != nil {
			return err
		}
		if paid >= booking.TotalPrice {
			return apperr.Conflict("booking is already fully paid")
		}
		created, err := s.payments.Create(ctx, tx, repo.PaymentCreateInput{
			BookingID:     booking.ID,
			Amount:        booking.TotalPrice,
			Method:        method,
			Status:        status,
			Reference:     ref,
			VerifiedBy:    verifiedBy,
			FailureReason: "",
		})
		if err != nil {
			return err
		}
		payment = created
		if status == model.PaymentPaid {
			if err := s.confirmBooking(ctx, tx, booking.ID); err != nil {
				return err
			}
		}
		return s.audit.Record(ctx, tx, audit.Entry{
			ActorID: &actor.ID, Action: "payment.create", EntityType: "payment", EntityID: &created.ID,
			Metadata: model.JSONMap{"booking_id": booking.ID.String(), "booking_code": booking.BookingCode,
				"amount": created.Amount, "method": created.Method, "status": string(created.Status)},
			IPAddress: meta.IP,
		})
	})
	if err != nil {
		return nil, err
	}
	return s.payments.FindByID(ctx, payment.ID)
}

// Get returns a payment of a booking the caller owns or operates on.
func (s *PaymentService) Get(ctx context.Context, actor *model.User, id uuid.UUID) (*model.Payment, error) {
	payment, err := s.payments.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	booking, err := s.bookings.FindByID(ctx, payment.BookingID)
	if err != nil {
		return nil, err
	}
	if err := requireOwnerOrStaff(actor, booking.CustomerID); err != nil {
		return nil, err
	}
	return payment, nil
}

// List returns payments for staff and administrators.
func (s *PaymentService) List(ctx context.Context, actor *model.User, f repo.PaymentFilter, page, perPage, offset int) (model.Page[model.Payment], error) {
	if err := requireStaff(actor); err != nil {
		return model.Page[model.Payment]{}, err
	}
	f.Limit, f.Offset = perPage, offset
	items, total, err := s.payments.List(ctx, f)
	if err != nil {
		return model.Page[model.Payment]{}, err
	}
	return model.NewPage(items, page, perPage, total), nil
}

// Settle applies an administrative payment decision. A paid payment confirms
// the booking it belongs to.
func (s *PaymentService) Settle(ctx context.Context, actor *model.User, id uuid.UUID, in PaymentSettleInput, meta RequestMeta) (*model.Payment, error) {
	if err := requireStaff(actor); err != nil {
		return nil, err
	}
	if !in.Status.Valid() {
		return nil, apperr.BadRequest("invalid payment status")
	}
	if _, err := s.payments.FindByID(ctx, id); err != nil {
		return nil, err
	}

	var from []model.PaymentStatus
	switch in.Status {
	case model.PaymentPaid, model.PaymentFailed:
		from = []model.PaymentStatus{model.PaymentUnpaid, model.PaymentPending}
	case model.PaymentRefunded:
		from = []model.PaymentStatus{model.PaymentPaid}
	default:
		return nil, apperr.BadRequest("payment can only be settled as PAID, FAILED or REFUNDED")
	}

	reference := strings.TrimSpace(in.Reference)
	var ref *string
	if reference != "" {
		ref = &reference
	}
	staff := actor.ID

	var updated *model.Payment
	err := s.db.InTx(ctx, func(tx pgx.Tx) error {
		settled, err := s.payments.Settle(ctx, tx, id, from, repo.SettleInput{
			Status:        in.Status,
			Reference:     ref,
			VerifiedBy:    &staff,
			FailureReason: strings.TrimSpace(in.FailureReason),
		})
		if err != nil {
			return err
		}
		updated = settled
		if settled.Status == model.PaymentPaid {
			if err := s.confirmBooking(ctx, tx, settled.BookingID); err != nil {
				return err
			}
		}
		return s.audit.Record(ctx, tx, audit.Entry{
			ActorID: &actor.ID, Action: "payment.settle", EntityType: "payment", EntityID: &settled.ID,
			Metadata: model.JSONMap{"booking_id": settled.BookingID.String(),
				"status": string(settled.Status), "amount": settled.Amount},
			IPAddress: meta.IP,
		})
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// confirmBooking promotes a pending booking once its payment is settled. A
// booking that is already confirmed or past its hold is left untouched.
func (s *PaymentService) confirmBooking(ctx context.Context, tx pgx.Tx, bookingID uuid.UUID) error {
	_, err := s.bookings.Transition(ctx, tx, bookingID,
		[]model.BookingStatus{model.BookingPending}, model.BookingConfirmed,
		repo.StatusPatch{ClearExpiry: true})
	if err != nil && apperr.Is(err, apperr.CodeConflict) {
		return nil
	}
	return err
}
