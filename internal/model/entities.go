package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Role is an authorization role of a user account.
type Role string

// Supported roles.
const (
	RoleCustomer Role = "CUSTOMER"
	RoleStaff    Role = "STAFF"
	RoleAdmin    Role = "ADMIN"
)

// AllRoles lists every valid role.
var AllRoles = []Role{RoleCustomer, RoleStaff, RoleAdmin}

// Valid reports whether the role is known.
func (r Role) Valid() bool {
	switch r {
	case RoleCustomer, RoleStaff, RoleAdmin:
		return true
	}
	return false
}

// IsStaffOrAdmin reports whether the role has operational privileges.
func (r Role) IsStaffOrAdmin() bool { return r == RoleStaff || r == RoleAdmin }

// ParseRole normalises a role string.
func ParseRole(s string) (Role, error) {
	r := Role(strings.ToUpper(strings.TrimSpace(s)))
	if !r.Valid() {
		return "", fmt.Errorf("invalid role %q: expected one of %s", s, joinRoles(AllRoles))
	}
	return r, nil
}

func joinRoles(roles []Role) string {
	parts := make([]string, len(roles))
	for i, r := range roles {
		parts[i] = string(r)
	}
	return strings.Join(parts, ", ")
}

// UserStatus is the lifecycle state of a user account.
type UserStatus string

// Supported user statuses.
const (
	UserStatusActive    UserStatus = "ACTIVE"
	UserStatusSuspended UserStatus = "SUSPENDED"
)

// User is an authenticated account.
type User struct {
	ID           uuid.UUID  `json:"id"`
	Email        string     `json:"email"`
	FullName     string     `json:"full_name"`
	Phone        *string    `json:"phone,omitempty"`
	PasswordHash string     `json:"-"`
	Role         Role       `json:"role"`
	Status       UserStatus `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
}

// IsActive reports whether the account may authenticate.
func (u User) IsActive() bool { return u.Status == UserStatusActive && u.DeletedAt == nil }

// Sport is a configurable sport type; it is never hardcoded in business logic.
type Sport struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// FacilityStatus is the operational state of a facility.
type FacilityStatus string

// Supported facility statuses.
const (
	FacilityActive      FacilityStatus = "ACTIVE"
	FacilityMaintenance FacilityStatus = "MAINTENANCE"
	FacilityClosed      FacilityStatus = "CLOSED"
)

// Valid reports whether the facility status is known.
func (s FacilityStatus) Valid() bool {
	switch s {
	case FacilityActive, FacilityMaintenance, FacilityClosed:
		return true
	}
	return false
}

// Facility is a bookable field/court belonging to a sport.
type Facility struct {
	ID          uuid.UUID      `json:"id"`
	SportID     uuid.UUID      `json:"sport_id"`
	Sport       *Sport         `json:"sport,omitempty"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Location    string         `json:"location"`
	Status      FacilityStatus `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// DayType selects which pricing rules apply on a date.
type DayType string

// Supported day types.
const (
	DayTypeWeekday DayType = "WEEKDAY"
	DayTypeWeekend DayType = "WEEKEND"
)

// Valid reports whether the day type is known.
func (d DayType) Valid() bool { return d == DayTypeWeekday || d == DayTypeWeekend }

// ParseDayType normalises a day type string.
func ParseDayType(s string) (DayType, error) {
	d := DayType(strings.ToUpper(strings.TrimSpace(s)))
	if !d.Valid() {
		return "", fmt.Errorf("invalid day_type %q: expected WEEKDAY or WEEKEND", s)
	}
	return d, nil
}

// PricingRule is a price window for a facility on a day type.
type PricingRule struct {
	ID          uuid.UUID `json:"id"`
	FacilityID  uuid.UUID `json:"facility_id"`
	DayType     DayType   `json:"day_type"`
	StartTime   Clock     `json:"start_time"`
	EndTime     Clock     `json:"end_time"`
	PricePerHour int64    `json:"price_per_hour"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// FacilityClosure marks a facility unavailable for a date or a time window.
type FacilityClosure struct {
	ID         uuid.UUID  `json:"id"`
	FacilityID uuid.UUID  `json:"facility_id"`
	StartDate  Date       `json:"start_date"`
	EndDate    Date       `json:"end_date"`
	StartTime  *Clock     `json:"start_time,omitempty"`
	EndTime    *Clock     `json:"end_time,omitempty"`
	Reason     string     `json:"reason"`
	CreatedBy  *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// CoversDate reports whether the closure applies to the given date.
func (c FacilityClosure) CoversDate(d Date) bool {
	return !d.Time.Before(c.StartDate.Time) && !d.Time.After(c.EndDate.Time)
}

// BookingStatus is the lifecycle state of a booking.
type BookingStatus string

// Supported booking statuses.
const (
	BookingPending   BookingStatus = "PENDING"
	BookingConfirmed BookingStatus = "CONFIRMED"
	BookingCancelled BookingStatus = "CANCELLED"
	BookingCompleted BookingStatus = "COMPLETED"
	BookingExpired   BookingStatus = "EXPIRED"
)

// Valid reports whether the booking status is known.
func (s BookingStatus) Valid() bool {
	switch s {
	case BookingPending, BookingConfirmed, BookingCancelled, BookingCompleted, BookingExpired:
		return true
	}
	return false
}

// ParseBookingStatus normalises a booking status string.
func ParseBookingStatus(s string) (BookingStatus, error) {
	v := BookingStatus(strings.ToUpper(strings.TrimSpace(s)))
	if !v.Valid() {
		return "", fmt.Errorf("invalid booking status %q", s)
	}
	return v, nil
}

// BlocksSlot reports whether the status reserves the slot against other bookings.
func (s BookingStatus) BlocksSlot() bool {
	return s == BookingPending || s == BookingConfirmed || s == BookingCompleted
}

// Booking is a reservation of a facility slot by a customer.
type Booking struct {
	ID           uuid.UUID     `json:"id"`
	BookingCode  string        `json:"booking_code"`
	CustomerID   uuid.UUID     `json:"customer_id"`
	FacilityID   uuid.UUID     `json:"facility_id"`
	BookingDate  Date          `json:"booking_date"`
	StartTime    Clock         `json:"start_time"`
	EndTime      Clock         `json:"end_time"`
	TotalPrice   int64         `json:"total_price"`
	Status       BookingStatus `json:"status"`
	Notes        string        `json:"notes,omitempty"`
	CancelledAt  *time.Time    `json:"cancelled_at,omitempty"`
	CancelledBy  *uuid.UUID    `json:"cancelled_by,omitempty"`
	CancelReason string        `json:"cancel_reason,omitempty"`
	ExpiresAt    *time.Time    `json:"expires_at,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`

	// Expanded relations, populated by list/detail queries.
	Customer *User     `json:"customer,omitempty"`
	Facility *Facility `json:"facility,omitempty"`
	Payment  *Payment  `json:"payment,omitempty"`
}

// PaymentStatus is the settlement state of a booking payment.
type PaymentStatus string

// Supported payment statuses.
const (
	PaymentUnpaid   PaymentStatus = "UNPAID"
	PaymentPending  PaymentStatus = "PENDING"
	PaymentPaid     PaymentStatus = "PAID"
	PaymentFailed   PaymentStatus = "FAILED"
	PaymentRefunded PaymentStatus = "REFUNDED"
)

// Valid reports whether the payment status is known.
func (s PaymentStatus) Valid() bool {
	switch s {
	case PaymentUnpaid, PaymentPending, PaymentPaid, PaymentFailed, PaymentRefunded:
		return true
	}
	return false
}

// ParsePaymentStatus normalises a payment status string.
func ParsePaymentStatus(s string) (PaymentStatus, error) {
	v := PaymentStatus(strings.ToUpper(strings.TrimSpace(s)))
	if !v.Valid() {
		return "", fmt.Errorf("invalid payment status %q", s)
	}
	return v, nil
}

// Payment records a payment attempt for a booking.
type Payment struct {
	ID            uuid.UUID     `json:"id"`
	BookingID     uuid.UUID     `json:"booking_id"`
	Amount        int64         `json:"amount"`
	Method        string        `json:"method"`
	Status        PaymentStatus `json:"status"`
	Reference     string        `json:"reference,omitempty"`
	PaidAt        *time.Time    `json:"paid_at,omitempty"`
	VerifiedBy    *uuid.UUID    `json:"verified_by,omitempty"`
	FailureReason string        `json:"failure_reason,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

// EventStatus is the lifecycle state of an event.
type EventStatus string

// Supported event statuses.
const (
	EventDraft     EventStatus = "DRAFT"
	EventPublished EventStatus = "PUBLISHED"
	EventOngoing   EventStatus = "ONGOING"
	EventCompleted EventStatus = "COMPLETED"
	EventCancelled EventStatus = "CANCELLED"
)

// Valid reports whether the event status is known.
func (s EventStatus) Valid() bool {
	switch s {
	case EventDraft, EventPublished, EventOngoing, EventCompleted, EventCancelled:
		return true
	}
	return false
}

// ParseEventStatus normalises an event status string.
func ParseEventStatus(s string) (EventStatus, error) {
	v := EventStatus(strings.ToUpper(strings.TrimSpace(s)))
	if !v.Valid() {
		return "", fmt.Errorf("invalid event status %q", s)
	}
	return v, nil
}

// RegistrationStatus is the state of an event registration.
type RegistrationStatus string

// Supported registration statuses.
const (
	RegistrationRegistered RegistrationStatus = "REGISTERED"
	RegistrationCancelled  RegistrationStatus = "CANCELLED"
)

// Event is an optional activity such as a tournament or clinic.
type Event struct {
	ID               uuid.UUID   `json:"id"`
	Title            string      `json:"title"`
	Description      string      `json:"description"`
	EventType        string      `json:"event_type"`
	StartAt          time.Time   `json:"start_at"`
	EndAt            time.Time   `json:"end_at"`
	Location         string      `json:"location"`
	Capacity         int         `json:"capacity"`
	Status           EventStatus `json:"status"`
	RegistrationFee  int64       `json:"registration_fee"`
	CreatedBy        *uuid.UUID  `json:"created_by,omitempty"`
	RegisteredCount  int         `json:"registered_count"`
	SeatsRemaining   int         `json:"seats_remaining"`
	CreatedAt        time.Time   `json:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at"`
}

// EventRegistration links a customer to an event.
type EventRegistration struct {
	ID            uuid.UUID          `json:"id"`
	EventID       uuid.UUID          `json:"event_id"`
	CustomerID    uuid.UUID          `json:"customer_id"`
	Status        RegistrationStatus `json:"status"`
	Notes         string             `json:"notes,omitempty"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
	CancelledAt   *time.Time         `json:"cancelled_at,omitempty"`

	// Expanded relations, populated by admin listing queries.
	Customer *User `json:"customer,omitempty"`
}

// ActorType identifies who performed an audited action.
type ActorType string

// Supported audit actor types.
const (
	ActorUser   ActorType = "USER"
	ActorSystem ActorType = "SYSTEM"
)

// AuditLog is an immutable record of a state changing action.
type AuditLog struct {
	ID         uuid.UUID  `json:"id"`
	ActorID    *uuid.UUID `json:"actor_id"`
	ActorType  ActorType  `json:"actor_type"`
	Action     string     `json:"action"`
	EntityType string     `json:"entity_type"`
	EntityID   *uuid.UUID `json:"entity_id,omitempty"`
	Metadata   JSONMap    `json:"metadata"`
	IPAddress  string     `json:"ip_address,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// RefreshToken is a rotating refresh token belonging to a user.
type RefreshToken struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	TokenHash string     `json:"-"`
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	UserAgent string     `json:"user_agent,omitempty"`
	IPAddress string     `json:"ip_address,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// Active reports whether the token is still usable at the given time.
func (t RefreshToken) Active(now time.Time) bool {
	return t.RevokedAt == nil && now.Before(t.ExpiresAt)
}

// Page is a keyset/count paginated result envelope.
type Page[T any] struct {
	Items      []T `json:"items"`
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	TotalItems int `json:"total_items"`
	TotalPages int `json:"total_pages"`
}

// NewPage builds a page envelope computing total pages.
func NewPage[T any](items []T, page, perPage, total int) Page[T] {
	if items == nil {
		items = []T{}
	}
	totalPages := 0
	if perPage > 0 {
		totalPages = (total + perPage - 1) / perPage
	}
	return Page[T]{Items: items, Page: page, PerPage: perPage, TotalItems: total, TotalPages: totalPages}
}
