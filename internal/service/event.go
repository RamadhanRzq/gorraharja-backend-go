package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/apperr"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/audit"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/database"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/repo"
)

// eventTransitions lists the status changes an operator may perform.
var eventTransitions = map[model.EventStatus][]model.EventStatus{
	model.EventDraft:     {model.EventPublished, model.EventCancelled},
	model.EventPublished: {model.EventOngoing, model.EventCancelled},
	model.EventOngoing:   {model.EventCompleted, model.EventCancelled},
}

// EventService manages the optional event module: publishing, capacity guarded
// registration and administrative oversight.
type EventService struct {
	db     *database.DB
	audit  *audit.Recorder
	events *repo.EventRepo
}

// EventInput is the create payload of an event.
type EventInput struct {
	Title           string            `json:"title" validate:"required,min=3,max=200"`
	Description     string            `json:"description" validate:"omitempty,max=2000"`
	EventType       string            `json:"event_type" validate:"omitempty,max=80"`
	StartAt         time.Time         `json:"start_at" validate:"required"`
	EndAt           time.Time         `json:"end_at" validate:"required"`
	Location        string            `json:"location" validate:"omitempty,max=200"`
	Capacity        int               `json:"capacity" validate:"required,gte=1,lte=100000"`
	Status          *model.EventStatus `json:"status" validate:"omitempty"`
	RegistrationFee int64             `json:"registration_fee" validate:"gte=0"`
}

// EventUpdateInput is the partial update payload of an event.
type EventUpdateInput struct {
	Title           *string           `json:"title" validate:"omitempty,min=3,max=200"`
	Description     *string           `json:"description" validate:"omitempty,max=2000"`
	EventType       *string           `json:"event_type" validate:"omitempty,max=80"`
	StartAt         *time.Time        `json:"start_at"`
	EndAt           *time.Time        `json:"end_at"`
	Location        *string           `json:"location" validate:"omitempty,max=200"`
	Capacity        *int              `json:"capacity" validate:"omitempty,gte=1,lte=100000"`
	Status          *model.EventStatus `json:"status"`
	RegistrationFee *int64            `json:"registration_fee" validate:"omitempty,gte=0"`
}

// EventRegisterInput is the payload of an event registration.
type EventRegisterInput struct {
	Notes string `json:"notes" validate:"omitempty,max=500"`
}

// List returns events visible to the caller: the public only sees published
// and ongoing events, staff and admins may use every filter.
func (s *EventService) List(ctx context.Context, actor *model.User, f repo.EventFilter, page, perPage, offset int) (model.Page[model.Event], error) {
	if !isStaff(actor) {
		f.PublishedOnly = true
		f.UpcomingOnly = true
		f.Status = nil
	}
	f.Search = strings.TrimSpace(f.Search)
	f.Limit, f.Offset = perPage, offset
	items, total, err := s.events.List(ctx, f)
	if err != nil {
		return model.Page[model.Event]{}, err
	}
	return model.NewPage(items, page, perPage, total), nil
}

// Get returns a single event. Draft events stay invisible to the public.
func (s *EventService) Get(ctx context.Context, actor *model.User, id uuid.UUID) (*model.Event, error) {
	event, err := s.events.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !isStaff(actor) && event.Status != model.EventPublished && event.Status != model.EventOngoing {
		return nil, apperr.NotFound("event not found")
	}
	return event, nil
}

// Create publishes a new event draft.
func (s *EventService) Create(ctx context.Context, actor *model.User, in EventInput, meta RequestMeta) (*model.Event, error) {
	if err := requireStaff(actor); err != nil {
		return nil, err
	}
	if !in.EndAt.After(in.StartAt) {
		return nil, apperr.BadRequest("end_at must be after start_at")
	}
	status := model.EventDraft
	if in.Status != nil {
		if !in.Status.Valid() {
			return nil, apperr.BadRequest("invalid event status")
		}
		status = *in.Status
	}
	createdBy := actor.ID
	event, err := s.events.Create(ctx, repo.EventCreateInput{
		Title:           strings.TrimSpace(in.Title),
		Description:     strings.TrimSpace(in.Description),
		EventType:       strings.TrimSpace(in.EventType),
		StartAt:         in.StartAt,
		EndAt:           in.EndAt,
		Location:        strings.TrimSpace(in.Location),
		Capacity:        in.Capacity,
		Status:          status,
		RegistrationFee: in.RegistrationFee,
		CreatedBy:       &createdBy,
	})
	if err != nil {
		return nil, err
	}
	s.audit.RecordDetached(ctx, audit.Entry{
		ActorID: &actor.ID, Action: "event.create", EntityType: "event", EntityID: &event.ID,
		Metadata:  model.JSONMap{"title": event.Title, "status": string(event.Status)},
		IPAddress: meta.IP,
	})
	return event, nil
}

// Update applies a partial update to an event, guarding the status lifecycle
// and refusing capacity cuts below the current registration count.
func (s *EventService) Update(ctx context.Context, actor *model.User, id uuid.UUID, in EventUpdateInput, meta RequestMeta) (*model.Event, error) {
	if err := requireStaff(actor); err != nil {
		return nil, err
	}
	current, err := s.events.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	start, end := current.StartAt, current.EndAt
	if in.StartAt != nil {
		start = *in.StartAt
	}
	if in.EndAt != nil {
		end = *in.EndAt
	}
	if !end.After(start) {
		return nil, apperr.BadRequest("end_at must be after start_at")
	}
	if in.Status != nil {
		if !in.Status.Valid() {
			return nil, apperr.BadRequest("invalid event status")
		}
		if *in.Status != current.Status && !allowedEventTransition(current.Status, *in.Status) {
			return nil, apperr.Conflict("event cannot move from " + string(current.Status) + " to " + string(*in.Status))
		}
	}
	if in.Capacity != nil && *in.Capacity < current.RegisteredCount {
		return nil, apperr.Conflict("capacity cannot be lower than the current registration count")
	}
	update := repo.EventUpdate{
		Title:           trimOrNil(in.Title),
		Description:     trimOrNil(in.Description),
		EventType:       trimOrNil(in.EventType),
		StartAt:         in.StartAt,
		EndAt:           in.EndAt,
		Location:        trimOrNil(in.Location),
		Capacity:        in.Capacity,
		Status:          in.Status,
		RegistrationFee: in.RegistrationFee,
	}
	event, err := s.events.Update(ctx, id, update)
	if err != nil {
		return nil, err
	}
	s.audit.RecordDetached(ctx, audit.Entry{
		ActorID: &actor.ID, Action: "event.update", EntityType: "event", EntityID: &event.ID,
		Metadata:  model.JSONMap{"title": event.Title, "status": string(event.Status)},
		IPAddress: meta.IP,
	})
	return event, nil
}

// Delete removes an event together with its registrations.
func (s *EventService) Delete(ctx context.Context, actor *model.User, id uuid.UUID, meta RequestMeta) error {
	if err := requireStaff(actor); err != nil {
		return err
	}
	if err := s.events.Delete(ctx, id); err != nil {
		return err
	}
	s.audit.RecordDetached(ctx, audit.Entry{
		ActorID: &actor.ID, Action: "event.delete", EntityType: "event", EntityID: &id,
		IPAddress: meta.IP,
	})
	return nil
}

// Register enrolls the caller in an event. The event row is locked inside the
// transaction so the capacity check and the insert serialize; the partial
// unique index on active registrations is the backstop against double
// enrollment under concurrency.
func (s *EventService) Register(ctx context.Context, actor *model.User, eventID uuid.UUID, in EventRegisterInput, meta RequestMeta) (*model.EventRegistration, error) {
	if err := requireActor(actor); err != nil {
		return nil, err
	}
	var reg *model.EventRegistration
	err := s.db.InTx(ctx, func(tx pgx.Tx) error {
		locked, err := s.events.LockEventRow(ctx, tx, eventID)
		if err != nil {
			return err
		}
		if locked.Status != model.EventPublished && locked.Status != model.EventOngoing {
			return apperr.Unprocessable("event is not open for registration")
		}
		count, err := s.events.CountRegistrations(ctx, tx, eventID)
		if err != nil {
			return err
		}
		if count >= locked.Capacity {
			return apperr.Conflict("event is fully booked")
		}
		created, err := s.events.Register(ctx, tx, eventID, actor.ID, strings.TrimSpace(in.Notes))
		if err != nil {
			if uniqueViolation(err, "event_registrations_active_key") {
				return apperr.Conflict("you are already registered for this event")
			}
			return err
		}
		reg = created
		return s.audit.Record(ctx, tx, audit.Entry{
			ActorID: &actor.ID, Action: "event.register", EntityType: "event_registration",
			EntityID:   &created.ID,
			Metadata:   model.JSONMap{"event_id": eventID.String()},
			IPAddress:  meta.IP,
		})
	})
	if err != nil {
		return nil, err
	}
	return reg, nil
}

// CancelRegistration revokes an enrollment. Customers may only revoke their
// own, staff and admins may revoke anyone's.
func (s *EventService) CancelRegistration(ctx context.Context, actor *model.User, eventID, customerID uuid.UUID, meta RequestMeta) error {
	if err := requireOwnerOrStaff(actor, customerID); err != nil {
		return err
	}
	return s.db.InTx(ctx, func(tx pgx.Tx) error {
		if err := s.events.CancelRegistration(ctx, tx, eventID, customerID); err != nil {
			return err
		}
		return s.audit.Record(ctx, tx, audit.Entry{
			ActorID: &actor.ID, Action: "event.unregister", EntityType: "event",
			EntityID:   &eventID,
			Metadata:   model.JSONMap{"event_id": eventID.String(), "customer_id": customerID.String()},
			IPAddress:  meta.IP,
		})
	})
}

// MyRegistrations returns the events the caller enrolled in.
func (s *EventService) MyRegistrations(ctx context.Context, actor *model.User, page, perPage, offset int) (model.Page[model.EventRegistration], error) {
	if err := requireActor(actor); err != nil {
		return model.Page[model.EventRegistration]{}, err
	}
	items, total, err := s.events.ListCustomerRegistrations(ctx, actor.ID, perPage, offset)
	if err != nil {
		return model.Page[model.EventRegistration]{}, err
	}
	return model.NewPage(items, page, perPage, total), nil
}

// ListRegistrations returns the enrollments of an event for operators.
func (s *EventService) ListRegistrations(ctx context.Context, actor *model.User, f repo.RegistrationFilter, page, perPage, offset int) (model.Page[model.EventRegistration], error) {
	if err := requireStaff(actor); err != nil {
		return model.Page[model.EventRegistration]{}, err
	}
	if _, err := s.events.FindByID(ctx, f.EventID); err != nil {
		return model.Page[model.EventRegistration]{}, err
	}
	f.Limit, f.Offset = perPage, offset
	items, total, err := s.events.ListRegistrations(ctx, f)
	if err != nil {
		return model.Page[model.EventRegistration]{}, err
	}
	return model.NewPage(items, page, perPage, total), nil
}

func allowedEventTransition(from, to model.EventStatus) bool {
	for _, next := range eventTransitions[from] {
		if next == to {
			return true
		}
	}
	return false
}

func isStaff(u *model.User) bool {
	return u != nil && u.IsActive() && u.Role.IsStaffOrAdmin()
}

func trimOrNil(s *string) *string {
	if s == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*s)
	return &trimmed
}
