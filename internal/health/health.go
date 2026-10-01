// Package health exposes the liveness/readiness probe of the API.
package health

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/database"
)

// Handler answers GET /health with the database reachability.
type Handler struct {
	db *database.DB
}

// New builds a health handler over the given pool.
func New(db *database.DB) *Handler {
	return &Handler{db: db}
}

// Handle reports "ok" when the database pings, "degraded" otherwise.
func (h *Handler) Handle(c *gin.Context) {
	if h == nil || h.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "database": "down"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	if err := h.db.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "database": "down"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "database": "up"})
}
