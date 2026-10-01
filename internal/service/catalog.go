package service

import (
	"context"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/apperr"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/audit"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/pricing"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/repo"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// SportService manages the sports offered by the venue.
type SportService struct {
	sports *repo.SportRepo
	audit  *audit.Recorder
}

// SportInput is the create payload of a sport.
type SportInput struct {
	Name        string
	Slug        string
	Description string
	IsActive    *bool
}

// SportUpdateInput is the partial update payload of a sport.
type SportUpdateInput struct {
	Name        *string
	Slug        *string
	Description *string
	IsActive    *bool
}

// List returns sports for the customer catalogue.
func (s *SportService) List(ctx context.Context, activeOnly bool, search string, page, perPage, offset int) (model.Page[model.Sport], error) {
	items, total, err := s.sports.List(ctx, repo.SportFilter{
		ActiveOnly: activeOnly,
		Search:     strings.TrimSpace(search),
		Limit:      perPage,
		Offset:     offset,
	})
	if err != nil {
		return model.Page[model.Sport]{}, err
	}
	return model.NewPage(items, page, perPage, total), nil
}

// Get returns a single sport.
func (s *SportService) Get(ctx context.Context, id uuid.UUID) (*model.Sport, error) {
	return s.sports.FindByID(ctx, id)
}

// Create adds a sport to the catalogue.
func (s *SportService) Create(ctx context.Context, actor *model.User, in SportInput, meta RequestMeta) (*model.Sport, error) {
	if err := requireAdmin(actor); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	slug := strings.ToLower(strings.TrimSpace(in.Slug))
	if name == "" {
		return nil, apperr.BadRequest("invalid sport", map[string]string{"name": "is required"})
	}
	if err := validateSlug(slug); err != nil {
		return nil, err
	}
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	sport, err := s.sports.Create(ctx, name, slug, strings.TrimSpace(in.Description), active)
	if err != nil {
		return nil, err
	}
	s.audit.RecordDetached(ctx, audit.Entry{
		ActorID: &actor.ID, Action: "sport.create", EntityType: "sport", EntityID: &sport.ID,
		Metadata: model.JSONMap{"name": sport.Name, "slug": sport.Slug}, IPAddress: meta.IP,
	})
	return sport, nil
}

// Update applies a partial update to a sport.
func (s *SportService) Update(ctx context.Context, actor *model.User, id uuid.UUID, in SportUpdateInput, meta RequestMeta) (*model.Sport, error) {
	if err := requireAdmin(actor); err != nil {
		return nil, err
	}
	if in.Slug != nil {
		slug := strings.ToLower(strings.TrimSpace(*in.Slug))
		if err := validateSlug(slug); err != nil {
			return nil, err
		}
		in.Slug = &slug
	}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return nil, apperr.BadRequest("invalid sport", map[string]string{"name": "is required"})
		}
		in.Name = &name
	}
	sport, err := s.sports.Update(ctx, id, repo.SportUpdate{
		Name:        in.Name,
		Slug:        in.Slug,
		Description: in.Description,
		IsActive:    in.IsActive,
	})
	if err != nil {
		return nil, err
	}
	s.audit.RecordDetached(ctx, audit.Entry{
		ActorID: &actor.ID, Action: "sport.update", EntityType: "sport", EntityID: &sport.ID,
		Metadata: model.JSONMap{"name": sport.Name}, IPAddress: meta.IP,
	})
	return sport, nil
}

// Delete removes a sport that no facility references.
func (s *SportService) Delete(ctx context.Context, actor *model.User, id uuid.UUID, meta RequestMeta) error {
	if err := requireAdmin(actor); err != nil {
		return err
	}
	if err := s.sports.Delete(ctx, id); err != nil {
		if fkViolation(err) {
			return apperr.Conflict("sport still has facilities and cannot be deleted")
		}
		return err
	}
	s.audit.RecordDetached(ctx, audit.Entry{
		ActorID: &actor.ID, Action: "sport.delete", EntityType: "sport", EntityID: &id, IPAddress: meta.IP,
	})
	return nil
}

func validateSlug(slug string) error {
	if len(slug) < 3 || len(slug) > 60 || !slugPattern.MatchString(slug) {
		return apperr.BadRequest("invalid sport", map[string]string{
			"slug": "must be 3-60 characters of lowercase letters, digits and single hyphens",
		})
	}
	return nil
}

// FacilityService manages the bookable facilities of the venue.
type FacilityService struct {
	facilities *repo.FacilityRepo
	sports     *repo.SportRepo
	audit      *audit.Recorder
}

// FacilityInput is the create payload of a facility.
type FacilityInput struct {
	SportID     uuid.UUID
	Name        string
	Description string
	Location    string
	Status      model.FacilityStatus
}

// FacilityUpdateInput is the partial update payload of a facility.
type FacilityUpdateInput struct {
	SportID     *uuid.UUID
	Name        *string
	Description *string
	Location    *string
	Status      *model.FacilityStatus
}

// List returns facilities, optionally filtered by sport, status or search term.
func (s *FacilityService) List(ctx context.Context, f repo.FacilityFilter, page, perPage, offset int) (model.Page[model.Facility], error) {
	f.Limit = perPage
	f.Offset = offset
	items, total, err := s.facilities.List(ctx, f)
	if err != nil {
		return model.Page[model.Facility]{}, err
	}
	return model.NewPage(items, page, perPage, total), nil
}

// Get returns a single facility.
func (s *FacilityService) Get(ctx context.Context, id uuid.UUID) (*model.Facility, error) {
	return s.facilities.FindByID(ctx, id)
}

// Create adds a facility to an existing sport.
func (s *FacilityService) Create(ctx context.Context, actor *model.User, in FacilityInput, meta RequestMeta) (*model.Facility, error) {
	if err := requireAdmin(actor); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, apperr.BadRequest("invalid facility", map[string]string{"name": "is required"})
	}
	sport, err := s.sports.FindByID(ctx, in.SportID)
	if err != nil {
		return nil, err
	}
	status := in.Status
	if status == "" {
		status = model.FacilityActive
	}
	if !status.Valid() {
		return nil, apperr.BadRequest("invalid facility status")
	}
	facility, err := s.facilities.Create(ctx, sport.ID, name, strings.TrimSpace(in.Description),
		strings.TrimSpace(in.Location), status)
	if err != nil {
		return nil, err
	}
	s.audit.RecordDetached(ctx, audit.Entry{
		ActorID: &actor.ID, Action: "facility.create", EntityType: "facility", EntityID: &facility.ID,
		Metadata: model.JSONMap{"name": facility.Name, "sport_id": sport.ID.String()}, IPAddress: meta.IP,
	})
	return facility, nil
}

// Update applies a partial update to a facility.
func (s *FacilityService) Update(ctx context.Context, actor *model.User, id uuid.UUID, in FacilityUpdateInput, meta RequestMeta) (*model.Facility, error) {
	if err := requireAdmin(actor); err != nil {
		return nil, err
	}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return nil, apperr.BadRequest("invalid facility", map[string]string{"name": "is required"})
		}
		in.Name = &name
	}
	if in.Status != nil && !in.Status.Valid() {
		return nil, apperr.BadRequest("invalid facility status")
	}
	if in.SportID != nil {
		if _, err := s.sports.FindByID(ctx, *in.SportID); err != nil {
			return nil, err
		}
	}
	facility, err := s.facilities.Update(ctx, id, repo.FacilityUpdate{
		SportID:     in.SportID,
		Name:        in.Name,
		Description: in.Description,
		Location:    in.Location,
		Status:      in.Status,
	})
	if err != nil {
		return nil, err
	}
	s.audit.RecordDetached(ctx, audit.Entry{
		ActorID: &actor.ID, Action: "facility.update", EntityType: "facility", EntityID: &facility.ID,
		Metadata: model.JSONMap{"name": facility.Name}, IPAddress: meta.IP,
	})
	return facility, nil
}

// Delete removes a facility that has no booking history.
func (s *FacilityService) Delete(ctx context.Context, actor *model.User, id uuid.UUID, meta RequestMeta) error {
	if err := requireAdmin(actor); err != nil {
		return err
	}
	if err := s.facilities.Delete(ctx, id); err != nil {
		if fkViolation(err) {
			return apperr.Conflict("facility has booking history and cannot be deleted")
		}
		return err
	}
	s.audit.RecordDetached(ctx, audit.Entry{
		ActorID: &actor.ID, Action: "facility.delete", EntityType: "facility", EntityID: &id, IPAddress: meta.IP,
	})
	return nil
}

// PricingService manages the per-facility price windows.
type PricingService struct {
	rules      *repo.PricingRuleRepo
	facilities *repo.FacilityRepo
	audit      *audit.Recorder
}

// PricingRuleInput is the create payload of a pricing rule.
type PricingRuleInput struct {
	DayType      model.DayType
	StartTime    model.Clock
	EndTime      model.Clock
	PricePerHour int64
}

// PricingRuleUpdateInput is the partial update payload of a pricing rule.
type PricingRuleUpdateInput struct {
	DayType      *model.DayType
	StartTime    *model.Clock
	EndTime      *model.Clock
	PricePerHour *int64
	IsActive     *bool
}

// List returns every pricing rule of a facility, including inactive ones.
func (s *PricingService) List(ctx context.Context, facilityID uuid.UUID) ([]model.PricingRule, error) {
	if _, err := s.facilities.FindByID(ctx, facilityID); err != nil {
		return nil, err
	}
	return s.rules.ListByFacility(ctx, facilityID, false)
}

// Create adds a price window after checking it does not overlap a sibling rule.
func (s *PricingService) Create(ctx context.Context, actor *model.User, facilityID uuid.UUID, in PricingRuleInput, meta RequestMeta) (*model.PricingRule, error) {
	if err := requireAdmin(actor); err != nil {
		return nil, err
	}
	if _, err := s.facilities.FindByID(ctx, facilityID); err != nil {
		return nil, err
	}
	if err := validateRuleWindow(in.DayType, in.StartTime, in.EndTime, in.PricePerHour); err != nil {
		return nil, err
	}
	existing, err := s.rules.ListByFacility(ctx, facilityID, false)
	if err != nil {
		return nil, err
	}
	candidate := append(append([]model.PricingRule{}, existing...), model.PricingRule{
		FacilityID: facilityID, DayType: in.DayType, StartTime: in.StartTime,
		EndTime: in.EndTime, PricePerHour: in.PricePerHour, IsActive: true,
	})
	if err := pricing.ValidateRules(candidate); err != nil {
		return nil, apperr.BadRequest(err.Error())
	}
	rule, err := s.rules.Create(ctx, facilityID, in.DayType, in.StartTime, in.EndTime, in.PricePerHour)
	if err != nil {
		return nil, err
	}
	s.audit.RecordDetached(ctx, audit.Entry{
		ActorID: &actor.ID, Action: "pricing_rule.create", EntityType: "pricing_rule", EntityID: &rule.ID,
		Metadata: model.JSONMap{"facility_id": facilityID.String(), "day_type": string(rule.DayType),
			"price_per_hour": rule.PricePerHour}, IPAddress: meta.IP,
	})
	return rule, nil
}

// Update applies a partial update to a pricing rule.
func (s *PricingService) Update(ctx context.Context, actor *model.User, facilityID, ruleID uuid.UUID, in PricingRuleUpdateInput, meta RequestMeta) (*model.PricingRule, error) {
	if err := requireAdmin(actor); err != nil {
		return nil, err
	}
	current, err := s.rules.FindByID(ctx, ruleID)
	if err != nil {
		return nil, err
	}
	if current.FacilityID != facilityID {
		return nil, apperr.NotFound("pricing rule not found")
	}
	dayType := current.DayType
	start, end := current.StartTime, current.EndTime
	price := current.PricePerHour
	if in.DayType != nil {
		dayType = *in.DayType
	}
	if in.StartTime != nil {
		start = *in.StartTime
	}
	if in.EndTime != nil {
		end = *in.EndTime
	}
	if in.PricePerHour != nil {
		price = *in.PricePerHour
	}
	if err := validateRuleWindow(dayType, start, end, price); err != nil {
		return nil, err
	}
	existing, err := s.rules.ListByFacility(ctx, facilityID, false)
	if err != nil {
		return nil, err
	}
	candidate := make([]model.PricingRule, 0, len(existing))
	for _, r := range existing {
		if r.ID == ruleID {
			r.DayType, r.StartTime, r.EndTime, r.PricePerHour = dayType, start, end, price
		}
		candidate = append(candidate, r)
	}
	if err := pricing.ValidateRules(candidate); err != nil {
		return nil, apperr.BadRequest(err.Error())
	}
	rule, err := s.rules.Update(ctx, ruleID, repo.PricingRuleUpdate{
		DayType:      in.DayType,
		StartTime:    in.StartTime,
		EndTime:      in.EndTime,
		PricePerHour: in.PricePerHour,
		IsActive:     in.IsActive,
	})
	if err != nil {
		return nil, err
	}
	s.audit.RecordDetached(ctx, audit.Entry{
		ActorID: &actor.ID, Action: "pricing_rule.update", EntityType: "pricing_rule", EntityID: &rule.ID,
		Metadata: model.JSONMap{"facility_id": facilityID.String(), "price_per_hour": rule.PricePerHour},
		IPAddress: meta.IP,
	})
	return rule, nil
}

// Delete removes a pricing rule.
func (s *PricingService) Delete(ctx context.Context, actor *model.User, facilityID, ruleID uuid.UUID, meta RequestMeta) error {
	if err := requireAdmin(actor); err != nil {
		return err
	}
	current, err := s.rules.FindByID(ctx, ruleID)
	if err != nil {
		return err
	}
	if current.FacilityID != facilityID {
		return apperr.NotFound("pricing rule not found")
	}
	if err := s.rules.Delete(ctx, ruleID); err != nil {
		return err
	}
	s.audit.RecordDetached(ctx, audit.Entry{
		ActorID: &actor.ID, Action: "pricing_rule.delete", EntityType: "pricing_rule", EntityID: &ruleID,
		Metadata: model.JSONMap{"facility_id": facilityID.String()}, IPAddress: meta.IP,
	})
	return nil
}

func validateRuleWindow(dayType model.DayType, start, end model.Clock, price int64) error {
	if !dayType.Valid() {
		return apperr.BadRequest("invalid day type")
	}
	if !start.Before(end) {
		return apperr.BadRequest("end_time must be after start_time")
	}
	if price < 0 {
		return apperr.BadRequest("price_per_hour must not be negative")
	}
	return nil
}

// ClosureService manages maintenance windows of a facility.
type ClosureService struct {
	closures   *repo.ClosureRepo
	facilities *repo.FacilityRepo
	audit      *audit.Recorder
}

// ClosureInput is the create payload of a facility closure.
type ClosureInput struct {
	StartDate model.Date
	EndDate   model.Date
	StartTime *model.Clock
	EndTime   *model.Clock
	Reason    string
}

// List returns the closures of a facility, optionally limited to a date range.
func (s *ClosureService) List(ctx context.Context, facilityID uuid.UUID, from, to *model.Date) ([]model.FacilityClosure, error) {
	if _, err := s.facilities.FindByID(ctx, facilityID); err != nil {
		return nil, err
	}
	return s.closures.ListByFacility(ctx, facilityID, from, to)
}

// Create registers a maintenance window.
func (s *ClosureService) Create(ctx context.Context, actor *model.User, facilityID uuid.UUID, in ClosureInput, meta RequestMeta) (*model.FacilityClosure, error) {
	if err := requireAdmin(actor); err != nil {
		return nil, err
	}
	if _, err := s.facilities.FindByID(ctx, facilityID); err != nil {
		return nil, err
	}
	if in.EndDate.Time.Before(in.StartDate.Time) {
		return nil, apperr.BadRequest("end_date must not be before start_date")
	}
	if (in.StartTime == nil) != (in.EndTime == nil) {
		return nil, apperr.BadRequest("start_time and end_time must be provided together")
	}
	if in.StartTime != nil && !in.StartTime.Before(*in.EndTime) {
		return nil, apperr.BadRequest("end_time must be after start_time")
	}
	closure, err := s.closures.Create(ctx, model.FacilityClosure{
		FacilityID: facilityID,
		StartDate:  in.StartDate,
		EndDate:    in.EndDate,
		StartTime:  in.StartTime,
		EndTime:    in.EndTime,
		Reason:     strings.TrimSpace(in.Reason),
		CreatedBy:  &actor.ID,
	})
	if err != nil {
		return nil, err
	}
	s.audit.RecordDetached(ctx, audit.Entry{
		ActorID: &actor.ID, Action: "facility_closure.create", EntityType: "facility_closure",
		EntityID: &closure.ID,
		Metadata: model.JSONMap{"facility_id": facilityID.String(),
			"start_date": closure.StartDate.String(), "end_date": closure.EndDate.String()},
		IPAddress: meta.IP,
	})
	return closure, nil
}

// Delete removes a maintenance window.
func (s *ClosureService) Delete(ctx context.Context, actor *model.User, facilityID, closureID uuid.UUID, meta RequestMeta) error {
	if err := requireAdmin(actor); err != nil {
		return err
	}
	current, err := s.closures.FindByID(ctx, closureID)
	if err != nil {
		return err
	}
	if current.FacilityID != facilityID {
		return apperr.NotFound("facility closure not found")
	}
	if err := s.closures.Delete(ctx, closureID); err != nil {
		return err
	}
	s.audit.RecordDetached(ctx, audit.Entry{
		ActorID: &actor.ID, Action: "facility_closure.delete", EntityType: "facility_closure",
		EntityID: &closureID, Metadata: model.JSONMap{"facility_id": facilityID.String()}, IPAddress: meta.IP,
	})
	return nil
}
