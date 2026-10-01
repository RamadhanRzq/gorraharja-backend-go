package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/httpx"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/repo"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/service"
)

// ListAdminBookings handles GET /admin/bookings.
func (h *Handler) ListAdminBookings(c *gin.Context) {
	user, err := h.actor(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	page, perPage, offset := h.page(c)
	f, ferr := adminBookingFilterFromQuery(c)
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

// UpdateBookingStatus handles PATCH /admin/bookings/:id/status.
func (h *Handler) UpdateBookingStatus(c *gin.Context) {
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
	var in service.BookingStatusInput
	if err := httpx.BindJSON(c, &in); err != nil {
		httpx.Fail(c, err)
		return
	}
	got, err := h.svc.Bookings.UpdateStatus(c.Request.Context(), user, id, in, h.meta(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, got)
}

// adminBookingFilterFromQuery parses every admin booking list filter.
func adminBookingFilterFromQuery(c *gin.Context) (repo.BookingFilter, error) {
	var f repo.BookingFilter
	customerID, err := optionalUUIDQuery(c, "customer_id")
	if err != nil {
		return f, err
	}
	facilityID, err := optionalUUIDQuery(c, "facility_id")
	if err != nil {
		return f, err
	}
	sportID, err := optionalUUIDQuery(c, "sport_id")
	if err != nil {
		return f, err
	}
	f.CustomerID, f.FacilityID, f.SportID = customerID, facilityID, sportID
	if raw := strings.TrimSpace(c.Query("status")); raw != "" {
		parsed, err := model.ParseBookingStatus(raw)
		if err != nil {
			return f, errInvalidBookingStatus()
		}
		f.Status = &parsed
	}
	f.Code = strings.TrimSpace(c.Query("code"))
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

// ListAdminPayments handles GET /admin/payments.
func (h *Handler) ListAdminPayments(c *gin.Context) {
	user, err := h.actor(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	page, perPage, offset := h.page(c)
	f, ferr := paymentFilterFromQuery(c)
	if ferr != nil {
		httpx.Fail(c, ferr)
		return
	}
	got, err := h.svc.Payments.List(c.Request.Context(), user, f, page, perPage, offset)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.List(c, got)
}

// SettlePayment handles PATCH /admin/payments/:id.
func (h *Handler) SettlePayment(c *gin.Context) {
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
	var in service.PaymentSettleInput
	if err := httpx.BindJSON(c, &in); err != nil {
		httpx.Fail(c, err)
		return
	}
	got, err := h.svc.Payments.Settle(c.Request.Context(), user, id, in, h.meta(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, got)
}

// paymentFilterFromQuery parses the admin payment list filters.
func paymentFilterFromQuery(c *gin.Context) (repo.PaymentFilter, error) {
	var f repo.PaymentFilter
	bookingID, err := optionalUUIDQuery(c, "booking_id")
	if err != nil {
		return f, err
	}
	customerID, err := optionalUUIDQuery(c, "customer_id")
	if err != nil {
		return f, err
	}
	f.BookingID, f.CustomerID = bookingID, customerID
	if raw := strings.TrimSpace(c.Query("status")); raw != "" {
		parsed, err := model.ParsePaymentStatus(raw)
		if err != nil {
			return f, errInvalidPaymentStatus()
		}
		f.Status = &parsed
	}
	if raw := strings.TrimSpace(c.Query("method")); raw != "" {
		method := strings.ToUpper(raw)
		f.Method = &method
	}
	return f, nil
}
