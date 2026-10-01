package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/httpx"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/repo"
)

// ListSports handles GET /sports.
func (h *Handler) ListSports(c *gin.Context) {
	page, perPage, offset := h.page(c)
	activeOnly := true
	if c.Query("active") != "" {
		activeOnly = httpx.BoolQuery(c, "active", true)
	}
	got, err := h.svc.Sports.List(c.Request.Context(), activeOnly, strings.TrimSpace(c.Query("search")), page, perPage, offset)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.List(c, got)
}

// GetSport handles GET /sports/:id.
func (h *Handler) GetSport(c *gin.Context) {
	id, err := httpx.UUIDParam(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	got, err := h.svc.Sports.Get(c.Request.Context(), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, got)
}

// ListFacilities handles GET /facilities.
func (h *Handler) ListFacilities(c *gin.Context) {
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
	activeOnly := httpx.BoolQuery(c, "active_only", false)
	if c.Query("active") != "" {
		activeOnly = httpx.BoolQuery(c, "active", false)
	}
	got, err := h.svc.Facilities.List(c.Request.Context(), repo.FacilityFilter{
		SportID:    sportID,
		Status:     status,
		ActiveOnly: activeOnly,
		Search:     strings.TrimSpace(c.Query("search")),
	}, page, perPage, offset)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.List(c, got)
}

// GetFacility handles GET /facilities/:id.
func (h *Handler) GetFacility(c *gin.Context) {
	id, err := httpx.UUIDParam(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	got, err := h.svc.Facilities.Get(c.Request.Context(), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, got)
}

// GetAvailability handles GET /availability?facility_id=&date=YYYY-MM-DD.
func (h *Handler) GetAvailability(c *gin.Context) {
	facilityID, err := optionalUUIDQuery(c, "facility_id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	if facilityID == nil {
		httpx.Fail(c, errMissingFacilityID())
		return
	}
	date, err := httpx.DateQuery(c, "date")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	got, err := h.svc.Availability.Get(c.Request.Context(), *facilityID, date)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, got)
}
