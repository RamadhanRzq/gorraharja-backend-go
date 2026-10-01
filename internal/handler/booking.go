package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/httpx"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/repo"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/service"
)

// CreateBooking handles POST /bookings.
func (h *Handler) CreateBooking(c *gin.Context) {
	user, err := h.actor(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var in service.BookingInput
	if err := httpx.BindJSON(c, &in); err != nil {
		httpx.Fail(c, err)
		return
	}
	got, err := h.svc.Bookings.Create(c.Request.Context(), user, in, h.meta(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, got)
}

// ListBookings handles GET /bookings. Customers only ever see their own
// reservations; the service enforces the scoping.
func (h *Handler) ListBookings(c *gin.Context) {
	user, err := h.actor(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	page, perPage, offset := h.page(c)
	f, ferr := bookingFilterFromQuery(c)
	if ferr != nil {
		httpx.Fail(c, ferr)
		return
	}
	got, err := h.svc.Bookings.List(c.Request.Context(), user, f, page, perPage, offset)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.List(c, got)
}

// GetBooking handles GET /bookings/:id.
func (h *Handler) GetBooking(c *gin.Context) {
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
	got, err := h.svc.Bookings.Get(c.Request.Context(), user, id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, got)
}

// CancelBooking handles PATCH /bookings/:id/cancel.
func (h *Handler) CancelBooking(c *gin.Context) {
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
	var in service.BookingCancelInput
	_ = c.ShouldBindJSON(&in)
	got, err := h.svc.Bookings.Cancel(c.Request.Context(), user, id, in, h.meta(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, got)
}

// bookingFilterFromQuery parses the customer booking list filters.
func bookingFilterFromQuery(c *gin.Context) (repo.BookingFilter, error) {
	var f repo.BookingFilter
	if raw := strings.TrimSpace(c.Query("status")); raw != "" {
		parsed, err := model.ParseBookingStatus(raw)
		if err != nil {
			return f, errInvalidBookingStatus()
		}
		f.Status = &parsed
	}
	if raw := strings.TrimSpace(c.Query("code")); raw != "" {
		f.Code = raw
	}
	from, err := optionalDateQuery(c, "date_from")
	if err != nil {
		return f, err
	}
	to, err := optionalDateQuery(c, "date_to")
	if err != nil {
		return f, err
	}
	f.DateFrom, f.DateTo = from, to
	return f, nil
}
