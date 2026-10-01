package handler

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/httpx"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/repo"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/service"
)

// sportRequest is the JSON body for creating/updating a sport. The service
// structs carry no JSON tags, so the handler owns the transport shape.
type sportRequest struct {
	Name        *string `json:"name"`
	Slug        *string `json:"slug"`
	Description *string `json:"description"`
	IsActive    *bool   `json:"is_active"`
}

// facilityRequest is the JSON body for creating/updating a facility.
type facilityRequest struct {
	SportID     *uuid.UUID            `json:"sport_id"`
	Name        *string               `json:"name"`
	Description *string               `json:"description"`
	Location    *string               `json:"location"`
	Status      *model.FacilityStatus `json:"status"`
}

// pricingRuleRequest is the JSON body for creating/updating a pricing rule.
type pricingRuleRequest struct {
	DayType      *model.DayType `json:"day_type"`
	StartTime    *model.Clock   `json:"start_time"`
	EndTime      *model.Clock   `json:"end_time"`
	PricePerHour *int64         `json:"price_per_hour"`
	IsActive     *bool          `json:"is_active"`
}

// closureRequest is the JSON body for creating a facility closure.
type closureRequest struct {
	StartDate model.Date   `json:"start_date"`
	EndDate   model.Date   `json:"end_date"`
	StartTime *model.Clock `json:"start_time"`
	EndTime   *model.Clock `json:"end_time"`
	Reason    string       `json:"reason"`
}

// ListAdminSports handles GET /admin/sports.
func (h *Handler) ListAdminSports(c *gin.Context) {
	user, err := h.actor(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	_ = user
	page, perPage, offset := h.page(c)
	activeOnly := httpx.BoolQuery(c, "active_only", false)
	if c.Query("active") != "" {
		activeOnly = httpx.BoolQuery(c, "active", false)
	}
	got, err := h.svc.Sports.List(c.Request.Context(), activeOnly, strings.TrimSpace(c.Query("search")), page, perPage, offset)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.List(c, got)
}

// CreateSport handles POST /admin/sports.
func (h *Handler) CreateSport(c *gin.Context) {
	user, err := h.actor(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var body sportRequest
	if err := httpx.BindJSON(c, &body); err != nil {
		httpx.Fail(c, err)
		return
	}
	in := service.SportInput{}
	if body.Name != nil {
		in.Name = *body.Name
	}
	if body.Slug != nil {
		in.Slug = *body.Slug
	}
	if body.Description != nil {
		in.Description = *body.Description
	}
	in.IsActive = body.IsActive
	got, err := h.svc.Sports.Create(c.Request.Context(), user, in, h.meta(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, got)
}

// UpdateSport handles PATCH /admin/sports/:id.
func (h *Handler) UpdateSport(c *gin.Context) {
	user, err := h.actor(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	id, err := httpx.UUIDParam(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var body sportRequest
	if err := httpx.BindJSON(c, &body); err != nil {
		httpx.Fail(c, err)
		return
	}
	got, err := h.svc.Sports.Update(c.Request.Context(), user, id, service.SportUpdateInput{
		Name:        body.Name,
		Slug:        body.Slug,
		Description: body.Description,
		IsActive:    body.IsActive,
	}, h.meta(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, got)
}

// DeleteSport handles DELETE /admin/sports/:id.
func (h *Handler) DeleteSport(c *gin.Context) {
	user, err := h.actor(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	id, err := httpx.UUIDParam(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.svc.Sports.Delete(c.Request.Context(), user, id, h.meta(c)); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

// ListAdminFacilities handles GET /admin/facilities.
func (h *Handler) ListAdminFacilities(c *gin.Context) {
	user, err := h.actor(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	_ = user
	page, perPage, offset := h.page(c)
	sportID, err := optionalUUIDQuery(c, "sport_id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var status *model.FacilityStatus
	if raw := strings.TrimSpace(c.Query("status")); raw != "" {
		parsed, perr := parseFacilityStatus(raw)
		if perr != nil {
			httpx.Fail(c, perr)
			return
		}
		status = &parsed
	}
	got, err := h.svc.Facilities.List(c.Request.Context(), repo.FacilityFilter{
		SportID:    sportID,
		Status:     status,
		ActiveOnly: httpx.BoolQuery(c, "active_only", false),
		Search:     strings.TrimSpace(c.Query("search")),
	}, page, perPage, offset)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.List(c, got)
}

// CreateFacility handles POST /admin/facilities.
func (h *Handler) CreateFacility(c *gin.Context) {
	user, err := h.actor(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var body facilityRequest
	if err := httpx.BindJSON(c, &body); err != nil {
		httpx.Fail(c, err)
		return
	}
	if body.SportID == nil {
		httpx.Fail(c, errMissingSportID())
		return
	}
	in := service.FacilityInput{SportID: *body.SportID}
	if body.Name != nil {
		in.Name = *body.Name
	}
	if body.Description != nil {
		in.Description = *body.Description
	}
	if body.Location != nil {
		in.Location = *body.Location
	}
	if body.Status != nil {
		in.Status = *body.Status
	}
	got, err := h.svc.Facilities.Create(c.Request.Context(), user, in, h.meta(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, got)
}

// UpdateFacility handles PATCH /admin/facilities/:id.
func (h *Handler) UpdateFacility(c *gin.Context) {
	user, err := h.actor(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	id, err := httpx.UUIDParam(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var body facilityRequest
	if err := httpx.BindJSON(c, &body); err != nil {
		httpx.Fail(c, err)
		return
	}
	got, err := h.svc.Facilities.Update(c.Request.Context(), user, id, service.FacilityUpdateInput{
		SportID:     body.SportID,
		Name:        body.Name,
		Description: body.Description,
		Location:    body.Location,
		Status:      body.Status,
	}, h.meta(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, got)
}

// DeleteFacility handles DELETE /admin/facilities/:id.
func (h *Handler) DeleteFacility(c *gin.Context) {
	user, err := h.actor(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	id, err := httpx.UUIDParam(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.svc.Facilities.Delete(c.Request.Context(), user, id, h.meta(c)); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

// ListPricingRules handles GET /admin/facilities/:id/pricing-rules.
func (h *Handler) ListPricingRules(c *gin.Context) {
	if _, err := h.actor(c); err != nil {
		httpx.Fail(c, err)
		return
	}
	facilityID, err := httpx.UUIDParam(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	got, err := h.svc.Pricing.List(c.Request.Context(), facilityID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, got)
}

// CreatePricingRule handles POST /admin/facilities/:id/pricing-rules.
func (h *Handler) CreatePricingRule(c *gin.Context) {
	user, err := h.actor(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	facilityID, err := httpx.UUIDParam(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var body pricingRuleRequest
	if err := httpx.BindJSON(c, &body); err != nil {
		httpx.Fail(c, err)
		return
	}
	if body.DayType == nil || body.StartTime == nil || body.EndTime == nil || body.PricePerHour == nil {
		httpx.Fail(c, errMissingPricingFields())
		return
	}
	got, err := h.svc.Pricing.Create(c.Request.Context(), user, facilityID, service.PricingRuleInput{
		DayType:      *body.DayType,
		StartTime:    *body.StartTime,
		EndTime:      *body.EndTime,
		PricePerHour: *body.PricePerHour,
	}, h.meta(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, got)
}

// UpdatePricingRule handles PATCH /admin/facilities/:id/pricing-rules/:ruleId.
func (h *Handler) UpdatePricingRule(c *gin.Context) {
	user, err := h.actor(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	facilityID, err := httpx.UUIDParam(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	ruleID, err := httpx.UUIDParam(c, "ruleId")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var body pricingRuleRequest
	if err := httpx.BindJSON(c, &body); err != nil {
		httpx.Fail(c, err)
		return
	}
	got, err := h.svc.Pricing.Update(c.Request.Context(), user, facilityID, ruleID, service.PricingRuleUpdateInput{
		DayType:      body.DayType,
		StartTime:    body.StartTime,
		EndTime:      body.EndTime,
		PricePerHour: body.PricePerHour,
		IsActive:     body.IsActive,
	}, h.meta(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, got)
}

// DeletePricingRule handles DELETE /admin/facilities/:id/pricing-rules/:ruleId.
func (h *Handler) DeletePricingRule(c *gin.Context) {
	user, err := h.actor(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	facilityID, err := httpx.UUIDParam(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	ruleID, err := httpx.UUIDParam(c, "ruleId")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.svc.Pricing.Delete(c.Request.Context(), user, facilityID, ruleID, h.meta(c)); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

// ListClosures handles GET /admin/facilities/:id/closures.
func (h *Handler) ListClosures(c *gin.Context) {
	if _, err := h.actor(c); err != nil {
		httpx.Fail(c, err)
		return
	}
	facilityID, err := httpx.UUIDParam(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	from, err := optionalDateQuery(c, "from")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	to, err := optionalDateQuery(c, "to")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	got, err := h.svc.Closures.List(c.Request.Context(), facilityID, from, to)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, got)
}

// CreateClosure handles POST /admin/facilities/:id/closures.
func (h *Handler) CreateClosure(c *gin.Context) {
	user, err := h.actor(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	facilityID, err := httpx.UUIDParam(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var body closureRequest
	if err := httpx.BindJSON(c, &body); err != nil {
		httpx.Fail(c, err)
		return
	}
	if body.StartDate.IsZero() || body.EndDate.IsZero() {
		httpx.Fail(c, errMissingClosureDates())
		return
	}
	got, err := h.svc.Closures.Create(c.Request.Context(), user, facilityID, service.ClosureInput{
		StartDate: body.StartDate,
		EndDate:   body.EndDate,
		StartTime: body.StartTime,
		EndTime:   body.EndTime,
		Reason:    body.Reason,
	}, h.meta(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, got)
}

// DeleteClosure handles DELETE /admin/facilities/:id/closures/:closureId.
func (h *Handler) DeleteClosure(c *gin.Context) {
	user, err := h.actor(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	facilityID, err := httpx.UUIDParam(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	closureID, err := httpx.UUIDParam(c, "closureId")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.svc.Closures.Delete(c.Request.Context(), user, facilityID, closureID, h.meta(c)); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}
