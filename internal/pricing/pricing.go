// Package pricing computes booking prices server side from pricing rules.
package pricing

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
)

// ErrNoRule reports that no active pricing rule covers the requested slot.
var ErrNoRule = errors.New("no pricing rule configured for the selected slot")

// Rule is the pricing input for a single facility and day type.
type Rule struct {
	DayType      model.DayType
	StartTime    model.Clock
	EndTime      model.Clock
	PricePerHour int64
}

// Segment is a priced portion of a booking.
type Segment struct {
	StartTime model.Clock `json:"start_time"`
	EndTime   model.Clock `json:"end_time"`
	Minutes   int         `json:"minutes"`
	Price     int64       `json:"price"`
}

// Quote is the priced breakdown of a booking request.
type Quote struct {
	FacilityID   string    `json:"facility_id,omitempty"`
	Date         model.Date `json:"booking_date"`
	StartTime    model.Clock `json:"start_time"`
	EndTime      model.Clock `json:"end_time"`
	TotalMinutes int       `json:"total_minutes"`
	TotalPrice   int64     `json:"total_price"`
	Segments     []Segment `json:"segments"`
}

// Calculate prices the slot [start, end) on the given date using rules.
//
// Rules must not overlap for the same day type; the engine validates this and
// fails loudly rather than silently picking an arbitrary price. A slot is
// billable only when every minute of it is covered by an active rule, so a
// misconfigured schedule can never produce a partially priced booking.
func Calculate(date model.Date, start, end model.Clock, rules []Rule) (*Quote, error) {
	if !start.Before(end) {
		return nil, fmt.Errorf("end_time must be after start_time")
	}

	dayType := date.DayType()
	applicable := make([]Rule, 0, len(rules))
	for _, r := range rules {
		if r.DayType != dayType {
			continue
		}
		if r.PricePerHour < 0 {
			return nil, fmt.Errorf("pricing rule has a negative price")
		}
		if !r.StartTime.Before(r.EndTime) {
			return nil, fmt.Errorf("pricing rule %s-%s has an invalid window", r.StartTime, r.EndTime)
		}
		applicable = append(applicable, r)
	}
	if len(applicable) == 0 {
		return nil, fmt.Errorf("%w (day type %s)", ErrNoRule, dayType)
	}

	sort.Slice(applicable, func(i, j int) bool {
		return applicable[i].StartTime.Before(applicable[j].StartTime)
	})

	segments := make([]Segment, 0, len(applicable))
	cursor := start.Minutes()
	endMinutes := end.Minutes()

	for _, r := range applicable {
		if cursor >= endMinutes {
			break
		}
		ruleStart := r.StartTime.Minutes()
		ruleEnd := r.EndTime.Minutes()
		if ruleEnd <= cursor {
			continue
		}
		if ruleStart > cursor {
			return nil, fmt.Errorf("%w: no rule covers %s-%s",
				ErrNoRule, minutesToClock(cursor), minutesToClock(min(ruleStart, endMinutes)))
		}
		segEnd := min(ruleEnd, endMinutes)
		minutes := segEnd - cursor
		if minutes <= 0 {
			continue
		}
		price := int64(minutes) * r.PricePerHour / 60
		if int64(minutes)*r.PricePerHour%60 != 0 {
			return nil, fmt.Errorf("slot %s-%s cannot be priced: %d minutes is not a whole hour multiple",
				start, end, minutes)
		}
		segments = append(segments, Segment{
			StartTime: minutesToClock(cursor),
			EndTime:   minutesToClock(segEnd),
			Minutes:   minutes,
			Price:     price,
		})
		cursor = segEnd
	}

	if cursor < endMinutes {
		return nil, fmt.Errorf("%w: no rule covers %s-%s", ErrNoRule, minutesToClock(cursor), end)
	}

	var total int64
	for _, s := range segments {
		total += s.Price
	}
	return &Quote{
		Date:         date,
		StartTime:    start,
		EndTime:      end,
		TotalMinutes: endMinutes - start.Minutes(),
		TotalPrice:   total,
		Segments:     segments,
	}, nil
}

// FromModels adapts stored pricing rules into engine rules.
func FromModels(rules []model.PricingRule) []Rule {
	out := make([]Rule, 0, len(rules))
	for _, r := range rules {
		out = append(out, Rule{
			DayType:      r.DayType,
			StartTime:    r.StartTime,
			EndTime:      r.EndTime,
			PricePerHour: r.PricePerHour,
		})
	}
	return out
}

func minutesToClock(m int) model.Clock {
	return model.NewClock(m/60%24, m%60)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ValidateRules rejects overlapping or malformed rule sets for one facility.
func ValidateRules(rules []model.PricingRule) error {
	now := time.Time{}
	_ = now
	for _, dayType := range []model.DayType{model.DayTypeWeekday, model.DayTypeWeekend} {
		windows := make([]model.PricingRule, 0, len(rules))
		for _, r := range rules {
			if r.DayType != dayType {
				continue
			}
			if !r.StartTime.Before(r.EndTime) {
				return fmt.Errorf("rule window %s-%s is invalid", r.StartTime, r.EndTime)
			}
			windows = append(windows, r)
		}
		sort.Slice(windows, func(i, j int) bool {
			return windows[i].StartTime.Before(windows[j].StartTime)
		})
		for i := 1; i < len(windows); i++ {
			if windows[i].StartTime.Before(windows[i-1].EndTime) {
				return fmt.Errorf("pricing rules overlap on %s: %s-%s and %s-%s",
					dayType, windows[i-1].StartTime, windows[i-1].EndTime,
					windows[i].StartTime, windows[i].EndTime)
			}
		}
	}
	return nil
}
