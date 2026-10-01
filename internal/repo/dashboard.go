package repo

import (
	"context"
	"time"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/database"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
)

// DashboardRepo aggregates the operational figures shown on the admin dashboard.
type DashboardRepo struct {
	db *database.DB
}

// NewDashboardRepo builds a dashboard repository.
func NewDashboardRepo(db *database.DB) *DashboardRepo { return &DashboardRepo{db: db} }

// DashboardStats is the dashboard projection.
type DashboardStats struct {
	BookingsToday    int   `json:"bookings_today"`
	BookingsPending  int   `json:"bookings_pending"`
	BookingsConfirmed int  `json:"bookings_confirmed"`
	ActiveFacilities int   `json:"active_facilities"`
	RevenueToday     int64 `json:"revenue_today"`
	RevenueMonth     int64 `json:"revenue_month"`
	UpcomingBookings int   `json:"upcoming_bookings"`
	UpcomingEvents   int   `json:"upcoming_events"`
	TotalCustomers   int   `json:"total_customers"`
	OccupancyToday   int   `json:"occupancy_today"`
}

// UpcomingBooking is a compact projection of the next reservations.
type UpcomingBooking struct {
	ID          string    `json:"id"`
	BookingCode string    `json:"booking_code"`
	Facility    string    `json:"facility"`
	Customer    string    `json:"customer"`
	Date        model.Date `json:"booking_date"`
	StartTime   model.Clock `json:"start_time"`
	EndTime     model.Clock `json:"end_time"`
	Status      string    `json:"status"`
}

// UpcomingEvent is a compact projection of the next events.
type UpcomingEvent struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	StartAt       time.Time `json:"start_at"`
	EndAt         time.Time `json:"end_at"`
	Capacity      int       `json:"capacity"`
	Registered    int       `json:"registered_count"`
	SeatsRemaining int      `json:"seats_remaining"`
	Status        string    `json:"status"`
}

// Stats returns the dashboard counters for the given local day.
func (r *DashboardRepo) Stats(ctx context.Context, today model.Date) (*DashboardStats, error) {
	var s DashboardStats
	err := r.db.Pool().QueryRow(ctx, `
		SELECT
		    (SELECT count(*) FROM bookings WHERE booking_date = $1
		        AND status IN ('PENDING', 'CONFIRMED', 'COMPLETED')),
		    (SELECT count(*) FROM bookings WHERE status = 'PENDING'),
		    (SELECT count(*) FROM bookings WHERE status = 'CONFIRMED'),
		    (SELECT count(*) FROM facilities WHERE status = 'ACTIVE'),
		    (SELECT COALESCE(sum(p.amount), 0) FROM payments p
		        JOIN bookings b ON b.id = p.booking_id
		        WHERE p.status = 'PAID' AND b.booking_date = $1),
		    (SELECT COALESCE(sum(p.amount), 0) FROM payments p
		        JOIN bookings b ON b.id = p.booking_id
		        WHERE p.status = 'PAID'
		          AND b.booking_date >= date_trunc('month', $1::date)
		          AND b.booking_date < date_trunc('month', $1::date) + interval '1 month'),
		    (SELECT count(*) FROM bookings
		        WHERE status IN ('PENDING', 'CONFIRMED')
		          AND (booking_date + start_time) >= now()),
		    (SELECT count(*) FROM events
		        WHERE status IN ('PUBLISHED', 'ONGOING') AND end_at >= now()),
		    (SELECT count(*) FROM users WHERE role = 'CUSTOMER' AND deleted_at IS NULL),
		    (SELECT count(*) FROM bookings WHERE booking_date = $1
		        AND status IN ('PENDING', 'CONFIRMED', 'COMPLETED'))`,
		today).
		Scan(&s.BookingsToday, &s.BookingsPending, &s.BookingsConfirmed, &s.ActiveFacilities,
			&s.RevenueToday, &s.RevenueMonth, &s.UpcomingBookings, &s.UpcomingEvents,
			&s.TotalCustomers, &s.OccupancyToday)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// UpcomingBookings returns the next reservations starting from a moment.
func (r *DashboardRepo) UpcomingBookings(ctx context.Context, limit int) ([]UpcomingBooking, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT b.id, b.booking_code, f.name, u.full_name, b.booking_date,
		    b.start_time, b.end_time, b.status
		FROM bookings b
		JOIN facilities f ON f.id = b.facility_id
		JOIN users u ON u.id = b.customer_id
		WHERE b.status IN ('PENDING', 'CONFIRMED')
		  AND (b.booking_date + b.start_time) >= now()
		ORDER BY b.booking_date, b.start_time
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []UpcomingBooking
	for rows.Next() {
		var b UpcomingBooking
		if err := rows.Scan(&b.ID, &b.BookingCode, &b.Facility, &b.Customer, &b.Date,
			&b.StartTime, &b.EndTime, &b.Status); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// UpcomingEvents returns the next published or ongoing events.
func (r *DashboardRepo) UpcomingEvents(ctx context.Context, limit int) ([]UpcomingEvent, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT e.id, e.title, e.start_at, e.end_at, e.capacity, e.status,
		    COALESCE(r.registered, 0)
		FROM events e
		LEFT JOIN (
		    SELECT event_id, count(*) AS registered
		    FROM event_registrations WHERE status = 'REGISTERED'
		    GROUP BY event_id
		) r ON r.event_id = e.id
		WHERE e.status IN ('PUBLISHED', 'ONGOING') AND e.end_at >= now()
		ORDER BY e.start_at
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []UpcomingEvent
	for rows.Next() {
		var e UpcomingEvent
		if err := rows.Scan(&e.ID, &e.Title, &e.StartAt, &e.EndAt, &e.Capacity,
			&e.Status, &e.Registered); err != nil {
			return nil, err
		}
		e.SeatsRemaining = e.Capacity - e.Registered
		out = append(out, e)
	}
	return out, rows.Err()
}
