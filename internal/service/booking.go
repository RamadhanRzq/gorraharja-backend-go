package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/apperr"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/audit"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/config"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/database"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/pricing"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/repo"
)

// bookingTransitions lists the status changes an administrator may perform.
var bookingTransitions = map[model.BookingStatus][]model.BookingStatus{
	model.BookingPending:   {model.BookingConfirmed, model.BookingCancelled, model.BookingExpired},
	model.BookingConfirmed: {model.BookingCompleted, model.BookingCancelled},
}

// BookingService applies the reservation rules: server side pricing, closure
// checks and the database exclusion constraint that makes double booking
// impossible even under concurrency.
type BookingService struct {
	db         *database.DB
	cfg        *config.Config
	audit      *audit.Recorder
	bookings   *repo.BookingRepo
	facilities *repo.FacilityRepo
	rules      *repo.PricingRuleRepo
	closures   *repo.ClosureRepo
}

// BookingInput is the create payload of a reservation.
type BookingInput struct {
	FacilityID  uuid.UUID   `json:"facility_id" validate:"required"`
	BookingDate model.Date  `json:"booking_date" validate:"required"`
	StartTime   model.Clock `json:"start_time" validate:"required"`
	EndTime     model.Clock `json:"end_time" validate:"required"`
	Notes       string      `json:"notes" validate:"omitempty,max=500"`
}

// BookingCancelInput carries the optional reason of a cancellation.
type BookingCancelInput struct {
	Reason string `json:"reason" validate:"omitempty,max=500"`
}

// BookingStatusInput is the administrative status transition payload.
type BookingStatusInput struct {
	Status model.BookingStatus `json:"status" validate:"required"`
	Reason string              `json:"reason" validate:"omitempty,max=500"`
}

// Create prices and persists a booking for the authenticated customer. The
// price is always computed from the configured pricing rules, never from the
// request body.
func (s *BookingService) Create(ctx context.Context, actor *model.User, in BookingInput, meta RequestMeta) (*model.Booking, error) {
	if err := requireActor(actor); err != nil {
		return nil, err
	}
	facility, err := s.facilities.FindByID(ctx, in.FacilityID)
	if err != nil {
		return nil, err
	}
	if facility.Status != model.FacilityActive {
		return nil, apperr.Unprocessable("facility is not open for booking")
	}
	if !in.StartTime.Before(in.EndTime) {
		return nil, apperr.BadRequest("end_time must be after start_time")
	}
	today := model.Today(s.cfg.Location())
	if in.BookingDate.Time.Before(today.Time) {
		return nil, apperr.BadRequest("booking_date must not be in the past")
	}
	if maxDate := today.AddDays(s.cfg.BookingMaxAdvanceDays); in.BookingDate.Time.After(maxDate.Time) {
		return nil, apperr.BadRequest(fmt.Sprintf("booking_date must be within %d days from today",
			s.cfg.BookingMaxAdvanceDays))
	}
	if err := s.checkClosures(ctx, facility.ID, in); err != nil {
		return nil, err
	}

	rules, err := s.rules.ListByFacility(ctx, facility.ID, true)
	if err != nil {
		return nil, err
	}
	quote, err := pricing.Calculate(in.BookingDate, in.StartTime, in.EndTime, pricing.FromModels(rules))
	if err != nil {
		if errors.Is(err, pricing.ErrNoRule) {
			return nil, apperr.Unprocessable(err.Error())
		}
		return nil, apperr.BadRequest(err.Error())
	}

	code, err := newBookingCode(in.BookingDate)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	expiresAt := time.Now().Add(s.cfg.BookingPaymentDeadline)

	var booking *model.Booking
	err = s.db.InTx(ctx, func(tx pgx.Tx) error {
		busy, err := s.bookings.OverlapsBusy(ctx, tx, facility.ID, in.BookingDate, in.StartTime, in.EndTime)
		if err != nil {
			return err
		}
		if busy {
			return apperr.Conflict("the selected slot is no longer available")
		}
		created, err := s.bookings.Create(ctx, tx, repo.BookingCreateInput{
			BookingCode: code,
			CustomerID:  actor.ID,
			FacilityID:  facility.ID,
			Date:        in.BookingDate,
			StartTime:   in.StartTime,
			EndTime:     in.EndTime,
			TotalPrice:  quote.TotalPrice,
			Notes:       strings.TrimSpace(in.Notes),
			ExpiresAt:   &expiresAt,
		})
		if err != nil {
			return err
		}
		booking = created
		return s.audit.Record(ctx, tx, audit.Entry{
			ActorID: &actor.ID, Action: "booking.create", EntityType: "booking", EntityID: &created.ID,
			Metadata: model.JSONMap{
				"booking_code": created.BookingCode,
				"facility_id":  facility.ID.String(),
				"total_price":  created.TotalPrice,
				"booking_date": created.BookingDate.String(),
				"start_time":   created.StartTime.String(),
				"end_time":     created.EndTime.String(),
			},
			IPAddress: meta.IP,
		})
	})
	if err != nil {
		return nil, err
	}
	return s.bookings.FindByID(ctx, booking.ID)
}

// List returns bookings visible to the caller: customers only ever see their
// own reservations, staff and admins may use every filter.
func (s *BookingService) List(ctx context.Context, actor *model.User, f repo.BookingFilter, page, perPage, offset int) (model.Page[model.Booking], error) {
	if err := requireActor(actor); err != nil {
		return model.Page[model.Booking]{}, err
	}
	if !actor.Role.IsStaffOrAdmin() {
		customer := actor.ID
		f.CustomerID = &customer
	}
	f.Limit, f.Offset = perPage, offset
	items, total, err := s.bookings.List(ctx, f)
	if err != nil {
		return model.Page[model.Booking]{}, err
	}
	return model.NewPage(items, page, perPage, total), nil
}

// Get returns a booking the caller owns or operates on.
func (s *BookingService) Get(ctx context.Context, actor *model.User, id uuid.UUID) (*model.Booking, error) {
	booking, err := s.bookings.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := requireOwnerOrStaff(actor, booking.CustomerID); err != nil {
		return nil, err
	}
	return booking, nil
}

// Cancel releases a pending or confirmed booking back into the schedule.
func (s *BookingService) Cancel(ctx context.Context, actor *model.User, id uuid.UUID, in BookingCancelInput, meta RequestMeta) (*model.Booking, error) {
	booking, err := s.bookings.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := requireOwnerOrStaff(actor, booking.CustomerID); err != nil {
		return nil, err
	}
	if booking.Status != model.BookingPending && booking.Status != model.BookingConfirmed {
		return nil, apperr.Conflict("booking cannot be cancelled in its current state")
	}
	if cutoff := s.cfg.BookingCancellationCutoff; cutoff > 0 {
		start := booking.BookingDate.AtIn(booking.StartTime, s.cfg.Location())
		if nowIn(s.cfg.Location()).Add(cutoff).After(start) {
			return nil, apperr.Unprocessable("booking can no longer be cancelled")
		}
	}

	now := time.Now()
	err = s.db.InTx(ctx, func(tx pgx.Tx) error {
		if _, err := s.bookings.Transition(ctx, tx, id,
			[]model.BookingStatus{model.BookingPending, model.BookingConfirmed},
			model.BookingCancelled,
			repo.StatusPatch{CancelledAt: &now, CancelledBy: &actor.ID,
				CancelReason: strings.TrimSpace(in.Reason), ClearExpiry: true}); err != nil {
			return err
		}
		return s.audit.Record(ctx, tx, audit.Entry{
			ActorID: &actor.ID, Action: "booking.cancel", EntityType: "booking", EntityID: &id,
			Metadata:  model.JSONMap{"booking_code": booking.BookingCode, "reason": strings.TrimSpace(in.Reason)},
			IPAddress: meta.IP,
		})
	})
	if err != nil {
		return nil, err
	}
	return s.bookings.FindByID(ctx, id)
}

// UpdateStatus performs an administrative status transition, refusing changes
// that are not part of the booking lifecycle.
func (s *BookingService) UpdateStatus(ctx context.Context, actor *model.User, id uuid.UUID, in BookingStatusInput, meta RequestMeta) (*model.Booking, error) {
	if err := requireStaff(actor); err != nil {
		return nil, err
	}
	booking, err := s.bookings.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	target := in.Status
	if !target.Valid() {
		return nil, apperr.BadRequest("invalid booking status")
	}
	allowed := false
	for _, candidate := range bookingTransitions[booking.Status] {
		if candidate == target {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, apperr.Conflict(fmt.Sprintf("booking cannot move from %s to %s", booking.Status, target))
	}

	patch := repo.StatusPatch{ClearExpiry: target != model.BookingPending}
	if target == model.BookingCancelled {
		now := time.Now()
		patch.CancelledAt = &now
		patch.CancelledBy = &actor.ID
		patch.CancelReason = strings.TrimSpace(in.Reason)
	}
	err = s.db.InTx(ctx, func(tx pgx.Tx) error {
		if _, err := s.bookings.Transition(ctx, tx, id, []model.BookingStatus{booking.Status}, target, patch); err != nil {
			return err
		}
		return s.audit.Record(ctx, tx, audit.Entry{
			ActorID: &actor.ID, Action: "booking.status_update", EntityType: "booking", EntityID: &id,
			Metadata: model.JSONMap{"booking_code": booking.BookingCode,
				"from": string(booking.Status), "to": string(target), "reason": strings.TrimSpace(in.Reason)},
			IPAddress: meta.IP,
		})
	})
	if err != nil {
		return nil, err
	}
	return s.bookings.FindByID(ctx, id)
}

// checkClosures rejects a slot that overlaps a maintenance window.
func (s *BookingService) checkClosures(ctx context.Context, facilityID uuid.UUID, in BookingInput) error {
	closures, err := s.closures.ListByFacility(ctx, facilityID, &in.BookingDate, &in.BookingDate)
	if err != nil {
		return err
	}
	for _, c := range closures {
		if !c.CoversDate(in.BookingDate) {
			continue
		}
		if c.StartTime == nil {
			return apperr.Unprocessable("facility is closed on the selected date")
		}
		if c.StartTime.Before(in.EndTime) && c.EndTime.After(in.StartTime) {
			return apperr.Unprocessable("facility is under maintenance during the selected time")
		}
	}
	return nil
}

// newBookingCode builds a human readable, effectively collision free code.
func newBookingCode(date model.Date) (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate booking code: %w", err)
	}
	return "BK-" + date.Format("20060102") + "-" + strings.ToUpper(hex.EncodeToString(buf)), nil
}
