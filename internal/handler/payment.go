package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/httpx"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/service"
)

// CreatePayment handles POST /payments.
func (h *Handler) CreatePayment(c *gin.Context) {
	user, err := h.actor(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var in service.PaymentInput
	if err := httpx.BindJSON(c, &in); err != nil {
		httpx.Fail(c, err)
		return
	}
	got, err := h.svc.Payments.Create(c.Request.Context(), user, in, h.meta(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, got)
}

// GetPayment handles GET /payments/:id.
func (h *Handler) GetPayment(c *gin.Context) {
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
	got, err := h.svc.Payments.Get(c.Request.Context(), user, id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, got)
}
