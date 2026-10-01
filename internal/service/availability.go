package service

import (
	"context"
	"sort"

	"github.com/google/uuid"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/apperr"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/config"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/repo"
)

// slotMinutes is the granularity of the bookable intervals offered to clients.
const slotMinutes = 60

// Slot statuses surfaced by the availability endpoint.
const (
	SlotAvailable   = "AVAILABLE"
	SlotBooked      = "BOOKED"
	SlotMaintenance = "MAINTENANCE"
	SlotClosed      = "CLOSED"
)

// AvailabilityService derives the bookable slots of a facility day from the
// configured pricing windows, the maintenance calendar and the live bookings.
type AvailabilityService struct {
	cfg        *config.Config
	facilities *repo.FacilityRepo
	rules      *repo.PricingRuleRepo
	closures   *repo.ClosureRepo
	bookings   *repo.BookingRepo
}

// AvailabilitySlot is one bookable interval of a facility day.
type AvailabilitySlot struct {
	StartTime model.Clock `json:"start_time"`
	EndTime   model.Clock `json:"end_time"`
	Status    string      `json:"status"`
	Price     int64       `json:"price"`
	BookingID string      `json:"booking_id,omitempty"`
}

// AvailabilityResult is the availability projection of a single facility day.
type AvailabilityResult struct {
	FacilityID   uuid.UUID            `json:"facility_id"`
	FacilityName string               `json:"facility_name"`
	Date         model.Date           `json:"date"`
	DayType      model.DayType        `json:"day_type"`
	Status       model.FacilityStatus `json:"status"`
	Slots        []AvailabilitySlot   `json:"slots"`
}

// Get computes the slots of a facility for a date. The schedule itself comes
// from the active pricing rules, so an unconfigured day yields no slots rather
// than invented ones.
func (s *AvailabilityService) Get(ctx context.Context, facilityID uuid.UUID, date model.Date) (*AvailabilityResult, error) {
	facility, err := s.facilities.FindByID(ctx, facilityID)
	if err != nil {
		return nil, err
	}
	today := model.Today(s.cfg.Location())
	if date.Time.Before(today.Time) {
		return nil, apperr.BadRequest("date must not be in the past")
	}

	rules, err := s.rules.ListByFacility(ctx, facilityID, true)
	if err != nil {
		return nil, err
	}
	closures, err := s.closures.ListByFacility(ctx, facilityID, &date, &date)
	if err != nil {
		return nil, err
	}
	busy, err := s.bookings.BusySlots(ctx, facilityID, date)
	if err != nil {
		return nil, err
	}

	dayType := date.DayType()
	slots := make([]AvailabilitySlot, 0, 16)
	for _, rule := range rules {
		if rule.DayType != dayType {
			continue
		}
		for start := rule.StartTime.Minutes(); start < rule.EndTime.Minutes(); start += slotMinutes {
			end := start + slotMinutes
			if ruleEnd := rule.EndTime.Minutes(); end > ruleEnd {
				end = ruleEnd
			}
			slots = append(slots, AvailabilitySlot{
				StartTime: clockAtMinutes(start),
				EndTime:   clockAtMinutes(end),
				Status:    SlotAvailable,
				Price:     int64(end-start) * rule.PricePerHour / 60,
			})
		}
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i].StartTime.Before(slots[j].StartTime) })

	for i := range slots {
		status, bookingID := s.resolveSlot(facility.Status, date, slots[i], closures, busy)
		slots[i].Status = status
		slots[i].BookingID = bookingID
	}

	return &AvailabilityResult{
		FacilityID:   facility.ID,
		FacilityName: facility.Name,
		Date:         date,
		DayType:      dayType,
		Status:       facility.Status,
		Slots:        slots,
	}, nil
}

// resolveSlot reports the state of one slot, giving facility-level states
// precedence over maintenance windows, which in turn precede live bookings.
func (s *AvailabilityService) resolveSlot(
	facilityStatus model.FacilityStatus,
	date model.Date,
	slot AvailabilitySlot,
	closures []model.FacilityClosure,
	busy []repo.BusySlot,
) (string, string) {
	switch facilityStatus {
	case model.FacilityClosed:
		return SlotClosed, ""
	case model.FacilityMaintenance:
		return SlotMaintenance, ""
	}
	for _, c := range closures {
		if !c.CoversDate(date) {
			continue
		}
		if c.StartTime == nil || (c.StartTime.Before(slot.EndTime) && c.EndTime.After(slot.StartTime)) {
			return SlotMaintenance, ""
		}
	}
	for _, b := range busy {
		if b.StartTime.Before(slot.EndTime) && b.EndTime.After(slot.StartTime) {
			return SlotBooked, b.BookingID.String()
		}
	}
	return SlotAvailable, ""
}

// clockAtMinutes converts minutes since midnight into a wall-clock time.
func clockAtMinutes(minutes int) model.Clock {
	return model.NewClock(minutes/60%24, minutes%60)
}
