package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/httpx"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/middleware"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/repo"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/service"
)

// ListEvents handles GET /events. The service hides drafts from the public.
func (h *Handler) ListEvents(c *gin.Context) {
	page, perPage, offset := h.page(c)
	f, err := eventFilterFromQuery(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	got, err := h.svc.Events.List(c.Request.Context(), middleware.UserOrNil(c), f, page, perPage, offset)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.List(c, got)
}

// GetEvent handles GET /events/:id.
func (h *Handler) GetEvent(c *gin.Context) {
	id, err := httpx.UUIDParam(c, "id")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	got, err := h.svc.Events.Get(c.Request.Context(), middleware.UserOrNil(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, got)
}

// RegisterForEvent handles POST /events/:id/register.
func (h *Handler) RegisterForEvent(c *gin.Context) {
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
	var in service.EventRegisterInput
	_ = c.ShouldBindJSON(&in)
	got, err := h.svc.Events.Register(c.Request.Context(), user, eventID, in, h.meta(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, got)
}

// CancelEventRegistration handles DELETE /events/:id/register. Customers
// cancel their own enrollment; staff and admins may target another customer
// via ?customer_id=.
func (h *Handler) CancelEventRegistration(c *gin.Context) {
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
	customerID := user.ID
	if raw := strings.TrimSpace(c.Query("customer_id")); raw != "" {
		parsed, perr := parseStrictUUID(raw, "customer_id")
		if perr != nil {
			httpx.Fail(c, perr)
			return
		}
		customerID = parsed
	}
	if err := h.svc.Events.CancelRegistration(c.Request.Context(), user, eventID, customerID, h.meta(c)); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

// eventFilterFromQuery parses the event list filters.
func eventFilterFromQuery(c *gin.Context) (repo.EventFilter, error) {
	var f repo.EventFilter
	if raw := strings.TrimSpace(c.Query("status")); raw != "" {
		parsed, err := model.ParseEventStatus(raw)
		if err != nil {
			return f, errInvalidEventStatus()
		}
		f.Status = &parsed
	}
	f.Search = strings.TrimSpace(c.Query("search"))
	return f, nil
}
