package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/httpx"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/service"
)

// Register handles POST /auth/register.
func (h *Handler) Register(c *gin.Context) {
	var in service.RegisterInput
	if err := httpx.BindJSON(c, &in); err != nil {
		httpx.Fail(c, err)
		return
	}
	res, err := h.svc.Auth.Register(c.Request.Context(), in, h.meta(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, res)
}

// Login handles POST /auth/login.
func (h *Handler) Login(c *gin.Context) {
	var in service.LoginInput
	if err := httpx.BindJSON(c, &in); err != nil {
		httpx.Fail(c, err)
		return
	}
	res, err := h.svc.Auth.Login(c.Request.Context(), in, h.meta(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, res)
}

// Refresh handles POST /auth/refresh.
func (h *Handler) Refresh(c *gin.Context) {
	var in service.RefreshInput
	if err := httpx.BindJSON(c, &in); err != nil {
		httpx.Fail(c, err)
		return
	}
	res, err := h.svc.Auth.Refresh(c.Request.Context(), in, h.meta(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, res)
}

// logoutInput carries the refresh token to revoke. The services treat logout
// as idempotent, so no field validation is applied here.
type logoutInput struct {
	RefreshToken string `json:"refresh_token"`
}

// Logout handles POST /auth/logout.
func (h *Handler) Logout(c *gin.Context) {
	var in logoutInput
	_ = c.ShouldBindJSON(&in)
	if err := h.svc.Auth.Logout(c.Request.Context(), strings.TrimSpace(in.RefreshToken), h.meta(c)); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

// Me handles GET /auth/me.
func (h *Handler) Me(c *gin.Context) {
	user, err := h.actor(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	got, err := h.svc.Auth.Me(c.Request.Context(), user)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, got)
}

// UpdateProfile handles PATCH /auth/me.
func (h *Handler) UpdateProfile(c *gin.Context) {
	user, err := h.actor(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var in service.ProfileUpdateInput
	if err := httpx.BindJSON(c, &in); err != nil {
		httpx.Fail(c, err)
		return
	}
	got, err := h.svc.Auth.UpdateProfile(c.Request.Context(), user, in, h.meta(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, got)
}

// ChangePassword handles POST /auth/me/password.
func (h *Handler) ChangePassword(c *gin.Context) {
	user, err := h.actor(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var in service.PasswordChangeInput
	if err := httpx.BindJSON(c, &in); err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.svc.Auth.ChangePassword(c.Request.Context(), user, in, h.meta(c)); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}
