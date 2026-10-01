package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/httpx"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/repo"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/service"
)

// ListUsers handles GET /admin/users.
func (h *Handler) ListUsers(c *gin.Context) {
	user, err := h.actor(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	page, perPage, offset := h.page(c)
	f, ferr := userFilterFromQuery(c)
	if ferr != nil {
		httpx.Fail(c, ferr)
		return
	}
	got, err := h.svc.Users.List(c.Request.Context(), user, f, page, perPage, offset)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.List(c, got)
}

// GetUser handles GET /admin/users/:id.
func (h *Handler) GetUser(c *gin.Context) {
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
	got, err := h.svc.Users.Get(c.Request.Context(), user, id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, got)
}

// UpdateUser handles PATCH /admin/users/:id.
func (h *Handler) UpdateUser(c *gin.Context) {
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
	var in service.UserAdminUpdateInput
	if err := httpx.BindJSON(c, &in); err != nil {
		httpx.Fail(c, err)
		return
	}
	got, err := h.svc.Users.Update(c.Request.Context(), user, id, in, h.meta(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, got)
}

// DeleteUser handles DELETE /admin/users/:id.
func (h *Handler) DeleteUser(c *gin.Context) {
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
	if err := h.svc.Users.Delete(c.Request.Context(), user, id, h.meta(c)); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

// userFilterFromQuery parses the admin user list filters.
func userFilterFromQuery(c *gin.Context) (repo.ListFilter, error) {
	var f repo.ListFilter
	if raw := strings.TrimSpace(c.Query("role")); raw != "" {
		parsed, err := model.ParseRole(raw)
		if err != nil {
			return f, errInvalidRole()
		}
		f.Role = &parsed
	}
	if raw := strings.TrimSpace(c.Query("status")); raw != "" {
		parsed, err := parseUserStatus(raw)
		if err != nil {
			return f, err
		}
		status := model.UserStatus(parsed)
		f.Status = &status
	}
	f.Search = strings.TrimSpace(c.Query("search"))
	return f, nil
}

// ListAuditLogs handles GET /admin/audit-logs.
func (h *Handler) ListAuditLogs(c *gin.Context) {
	user, err := h.actor(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	page, perPage, offset := h.page(c)
	f, ferr := auditFilterFromQuery(c)
	if ferr != nil {
		httpx.Fail(c, ferr)
		return
	}
	got, err := h.svc.Audits.List(c.Request.Context(), user, f, page, perPage, offset)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.List(c, got)
}

// auditFilterFromQuery parses the audit log list filters.
func auditFilterFromQuery(c *gin.Context) (repo.AuditFilter, error) {
	var f repo.AuditFilter
	actorID, err := optionalUUIDQuery(c, "actor_id")
	if err != nil {
		return f, err
	}
	entityID, err := optionalUUIDQuery(c, "entity_id")
	if err != nil {
		return f, err
	}
	f.ActorID, f.EntityID = actorID, entityID
	f.Action = strings.TrimSpace(c.Query("action"))
	f.EntityType = strings.TrimSpace(c.Query("entity_type"))
	return f, nil
}

// GetDashboard handles GET /admin/dashboard.
func (h *Handler) GetDashboard(c *gin.Context) {
	user, err := h.actor(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	got, err := h.svc.Dashboard.Overview(c.Request.Context(), user, httpx.IntQuery(c, "days", 14))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, got)
}
