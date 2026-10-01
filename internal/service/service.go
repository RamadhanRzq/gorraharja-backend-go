// Package service holds the business rules of the booking system. Services
// depend on repositories and configuration only, never on HTTP, which keeps the
// rules unit testable and the handlers thin.
package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/apperr"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/audit"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/auth"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/config"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/database"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/repo"
)

// RequestMeta carries request scoped metadata used for auditing and refresh
// token bookkeeping.
type RequestMeta struct {
	IP        string
	UserAgent string
}

// Deps are the collaborators shared by the application services.
type Deps struct {
	DB     *database.DB
	Cfg    *config.Config
	Log    *slog.Logger
	Audit  *audit.Recorder
	Tokens *auth.Manager

	Users         *repo.UserRepo
	RefreshTokens *repo.RefreshTokenRepo
	Sports        *repo.SportRepo
	Facilities    *repo.FacilityRepo
	PricingRules  *repo.PricingRuleRepo
	Closures      *repo.ClosureRepo
	Bookings      *repo.BookingRepo
	Payments      *repo.PaymentRepo
	Events        *repo.EventRepo
	Dashboard     *repo.DashboardRepo
	Audits        *repo.AuditRepo
}

// Services bundles every application service behind a single constructor.
type Services struct {
	deps Deps

	Auth         *AuthService
	Sports       *SportService
	Facilities   *FacilityService
	Pricing      *PricingService
	Closures     *ClosureService
	Availability *AvailabilityService
	Bookings     *BookingService
	Payments     *PaymentService
	Events       *EventService
	Dashboard    *DashboardService
	Users        *UserService
	Audits       *AuditService
	Scheduler    *Scheduler
}

// New wires the services over the given dependencies.
func New(d Deps) *Services {
	s := &Services{deps: d}
	s.Auth = &AuthService{db: d.DB, cfg: d.Cfg, log: d.Log, audit: d.Audit,
		tokens: d.Tokens, users: d.Users, refresh: d.RefreshTokens}
	s.Sports = &SportService{sports: d.Sports, audit: d.Audit}
	s.Facilities = &FacilityService{facilities: d.Facilities, sports: d.Sports, audit: d.Audit}
	s.Pricing = &PricingService{rules: d.PricingRules, facilities: d.Facilities, audit: d.Audit}
	s.Closures = &ClosureService{closures: d.Closures, facilities: d.Facilities, audit: d.Audit}
	s.Availability = &AvailabilityService{cfg: d.Cfg, facilities: d.Facilities,
		rules: d.PricingRules, closures: d.Closures, bookings: d.Bookings}
	s.Bookings = &BookingService{db: d.DB, cfg: d.Cfg, audit: d.Audit,
		bookings: d.Bookings, facilities: d.Facilities, rules: d.PricingRules, closures: d.Closures}
	s.Payments = &PaymentService{db: d.DB, audit: d.Audit, payments: d.Payments, bookings: d.Bookings}
	s.Events = &EventService{db: d.DB, audit: d.Audit, events: d.Events}
	s.Dashboard = &DashboardService{dashboard: d.Dashboard, cfg: d.Cfg}
	s.Users = &UserService{users: d.Users, refresh: d.RefreshTokens, audit: d.Audit}
	s.Audits = &AuditService{audits: d.Audits}
	s.Scheduler = &Scheduler{cfg: d.Cfg, log: d.Log, bookings: d.Bookings, refresh: d.RefreshTokens}
	return s
}

// LoadUser resolves a persisted user by identifier. It satisfies
// middleware.UserLoader so the HTTP layer reuses a single lookup path.
func (s *Services) LoadUser(ctx context.Context, id string) (*model.User, error) {
	uid, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return nil, apperr.Unauthorized("invalid access token")
	}
	return s.deps.Users.FindByID(ctx, uid)
}

// requireActor rejects anonymous callers.
func requireActor(u *model.User) error {
	if u == nil {
		return apperr.Unauthorized("authentication is required")
	}
	if !u.IsActive() {
		return apperr.Forbidden("account is not active")
	}
	return nil
}

// requireStaff rejects callers that are neither staff nor admin.
func requireStaff(u *model.User) error {
	if err := requireActor(u); err != nil {
		return err
	}
	if !u.Role.IsStaffOrAdmin() {
		return apperr.Forbidden("you do not have access to this resource")
	}
	return nil
}

// requireAdmin rejects callers that are not administrators.
func requireAdmin(u *model.User) error {
	if err := requireActor(u); err != nil {
		return err
	}
	if u.Role != model.RoleAdmin {
		return apperr.Forbidden("administrator privileges are required")
	}
	return nil
}

// requireOwnerOrStaff allows the owner of a resource plus staff and admins.
func requireOwnerOrStaff(u *model.User, ownerID uuid.UUID) error {
	if err := requireActor(u); err != nil {
		return err
	}
	if u.ID == ownerID || u.Role.IsStaffOrAdmin() {
		return nil
	}
	return apperr.Forbidden("you do not have access to this resource")
}

// uniqueViolation reports whether err is a unique index violation on the given
// constraint name.
func uniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}

// fkViolation reports whether err is a foreign key violation, which the delete
// paths surface as a conflict because dependent rows still reference the row.
func fkViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

// nowIn returns the current instant in the configured timezone.
func nowIn(loc *time.Location) time.Time { return time.Now().In(loc) }
