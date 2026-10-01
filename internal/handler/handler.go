// Package handler holds the thin HTTP adapters over the application services.
// Handlers parse transport concerns (paths, query strings, bodies) and leave
// every business rule to the service layer, which is the single validation
// authority.
package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/apperr"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/httpx"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/middleware"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/service"
)

// Handler adapts service calls to Gin handlers.
type Handler struct {
	svc *service.Services
}

// New builds a handler over the application services.
func New(svc *service.Services) *Handler {
	return &Handler{svc: svc}
}

// actor returns the authenticated caller or fails the request.
func (h *Handler) actor(c *gin.Context) (*model.User, error) {
	return middleware.User(c)
}

// meta carries request scoped auditing metadata into the services.
func (h *Handler) meta(c *gin.Context) service.RequestMeta {
	return service.RequestMeta{
		IP:        middleware.ClientIP(c),
		UserAgent: c.Request.UserAgent(),
	}
}

// page parses pagination parameters into page, per-page and offset.
func (h *Handler) page(c *gin.Context) (page, perPage, offset int) {
	p := httpx.ParsePagination(c)
	return p.Page, p.PerPage, p.Offset
}

// optionalUUIDQuery parses an optional UUID query parameter.
func optionalUUIDQuery(c *gin.Context, key string) (*uuid.UUID, error) {
	raw := c.Query(key)
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, apperr.BadRequest("query parameter '" + key + "' must be a valid UUID")
	}
	return &id, nil
}

// optionalDateQuery parses an optional YYYY-MM-DD query parameter.
func optionalDateQuery(c *gin.Context, key string) (*model.Date, error) {
	raw := c.Query(key)
	if raw == "" {
		return nil, nil
	}
	d, err := model.ParseDate(raw)
	if err != nil {
		return nil, apperr.BadRequest("query parameter '" + key + "' must be a valid date (YYYY-MM-DD)")
	}
	return &d, nil
}
