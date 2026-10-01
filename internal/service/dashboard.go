package service

import (
	"context"
	"time"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/apperr"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/config"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/repo"
)

// DashboardService aggregates the operational figures of the admin dashboard.
type DashboardService struct {
	dashboard *repo.DashboardRepo
	payments  *repo.PaymentRepo
	cfg       *config.Config
}

// DashboardOverview is the dashboard payload: counters, upcoming items and a
// revenue series.
type DashboardOverview struct {
	Stats            *repo.DashboardStats       `json:"stats"`
	UpcomingBookings []repo.UpcomingBooking     `json:"upcoming_bookings"`
	UpcomingEvents   []repo.UpcomingEvent       `json:"upcoming_events"`
	RevenueSeries    []repo.RevenueSummary      `json:"revenue_series"`
	From             model.Date                 `json:"from"`
	To               model.Date                 `json:"to"`
}

// Overview returns counters, upcoming items and the paid-revenue series of the
// last days days (default 14, capped at 90) for staff and admins.
func (s *DashboardService) Overview(ctx context.Context, actor *model.User, days int) (*DashboardOverview, error) {
	if err := requireStaff(actor); err != nil {
		return nil, err
	}
	if days <= 0 {
		days = 14
	}
	if days > 90 {
		days = 90
	}
	today := model.Today(s.cfg.Location())
	from := today.AddDays(-(days - 1))

	stats, err := s.dashboard.Stats(ctx, today)
	if err != nil {
		return nil, err
	}
	bookings, err := s.dashboard.UpcomingBookings(ctx, 10)
	if err != nil {
		return nil, err
	}
	events, err := s.dashboard.UpcomingEvents(ctx, 5)
	if err != nil {
		return nil, err
	}
	series, err := s.payments.RevenueByDay(ctx, from, today)
	if err != nil {
		return nil, err
	}
	return &DashboardOverview{
		Stats:            stats,
		UpcomingBookings: bookings,
		UpcomingEvents:   events,
		RevenueSeries:    series,
		From:             from,
		To:               today,
	}, nil
}
// Revenue returns the paid-revenue series between from and to, capped at a 366
// day window.
func (s *DashboardService) Revenue(ctx context.Context, actor *model.User, from, to model.Date) ([]repo.RevenueSummary, error) {
	if err := requireStaff(actor); err != nil {
		return nil, err
	}
	if from.IsZero() || to.IsZero() {
		return nil, apperr.BadRequest("from and to dates are required")
	}
	if to.Time.Before(from.Time) {
		return nil, apperr.BadRequest("to must not be before from")
	}
	if to.Time.Sub(from.Time) > 366*24*time.Hour {
		return nil, apperr.BadRequest("date range must not exceed 366 days")
	}
	return s.payments.RevenueByDay(ctx, from, to)
}
