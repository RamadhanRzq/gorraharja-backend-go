package router

import (
	"booking-manager/internal/handler"
	"booking-manager/internal/middleware"

	"github.com/gin-gonic/gin"
)

// New menyusun engine Gin beserta seluruh route API.
// adminKey cocok -> peran "admin" (akses penuh); userKey cocok -> peran
// "user" (boleh baca + tambah; dilarang PUT/PATCH/DELETE).
func New(h *handler.BookingHandler, adminKey, userKey string) *gin.Engine {
	r := gin.New()
	r.Use(middleware.CORS())
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(gin.Logger())
	r.Use(middleware.Auth(adminKey, userKey))

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	v1 := r.Group("/api/v1")
	{
		v1.GET("/me", h.Me)
		// Export didaftarkan sebelum :id agar tidak tertelan route param.
		v1.GET("/bookings/export/pdf", h.ExportPDF)
		v1.POST("/bookings", h.Create)
		v1.GET("/bookings", h.List)
		v1.GET("/bookings/:id", h.Detail)
		admin := v1.Group("/bookings", middleware.RequireAdmin())
		{
			admin.PUT("/:id", h.Put)
			admin.PATCH("/:id", h.Patch)
			admin.DELETE("/:id", h.Delete)
		}
	}
	return r
}

// NewWithAPIKey kompatibilitas legacy: satu kunci = admin penuh.
func NewWithAPIKey(h *handler.BookingHandler, apiKey string) *gin.Engine {
	return New(h, apiKey, "")
}
