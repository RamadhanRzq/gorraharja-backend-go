package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/httpx"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/repo"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/service"
)

// ListAdminEvents handles GET /admin/events.
func (h *Handler) ListAdminEvents(c *gin.Context) {
	user, err := h.actor(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	page, perPage, offset := h.page(c)
	f, ferr := eventFilterFromQuery(c)
	if ferr != nil {
		httpx.Fail(c, ferr)
		return
	}
	got, err := h.svc.Events.List(c.Request.Context(), user, f, page, perPage, offset)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.List(c, got)
}

// CreateEvent handles POST /admin/events.
func (h *Handler) CreateEvent(c *gin.Context) {
	user, err := h.actor(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var in service.EventInput
	if err := httpx.BindJSON(c, &in); err != nil {
		httpx.Fail(c, err)
		return
	}
	got, err := h.svc.Events.Create(c.Request.Context(), user, in, h.meta(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, got)
}

// UpdateEvent handles PATCH /admin/events/:id.
func (h *Handler) UpdateEvent(c *gin.Context) {
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
	var in service.EventUpdateInput
	if err := httpx.BindJSON(c, &in); err != nil {
		httpx.Fail(c, err)
		return
	}
	got, err := h.svc.Events.Update(c.Request.Context(), user, id, in, h.meta(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, got)
}

// DeleteEvent handles DELETE /admin/events/:id.
func (h *Handler) DeleteEvent(c *gin.Context) {
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
	if err := h.svc.Events.Delete(c.Request.Context(), user, id, h.meta(c)); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

// ListEventRegistrations handles GET /admin/events/:id/registrations.
func (h *Handler) ListEventRegistrations(c *gin.Context) {
	user, err := h.actor(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	eventID, err := httpx.UUIDParam(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	page, perPage, offset := h.page(c)
	f := repo.RegistrationFilter{EventID: eventID}
	if raw := strings.TrimSpace(c.Query("status")); raw != "" {
		parsed, perr := parseRegistrationStatus(raw)
		if perr != nil {
			httpx.Fail(c, perr)
			return
		}
		f.Status = &parsed
	}
	got, err := h.svc.Events.ListRegistrations(c.Request.Context(), user, f, page, perPage, offset)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.List(c, got)
}

// ListMyRegistrations handles GET /events/registrations/mine: the events the
// caller enrolled in.
func (h *Handler) ListMyRegistrations(c *gin.Context) {
	user, err := h.actor(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	page, perPage, offset := h.page(c)
	got, err := h.svc.Events.MyRegistrations(c.Request.Context(), user, page, perPage, offset)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.List(c, got)
}
