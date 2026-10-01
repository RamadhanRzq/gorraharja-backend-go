// Package server assembles the Gin engine and registers every route.
package server

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/auth"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/config"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/database"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/handler"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/health"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/middleware"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/service"
)

// Deps are the collaborators needed to build the router.
type Deps struct {
	Config   *config.Config
	Log      *slog.Logger
	DB       *database.DB
	Tokens   *auth.Manager
	Services *service.Services
}

func New(d Deps) *gin.Engine {
	log := d.Log
	if log == nil {
		log = slog.Default()
	}
	var origins []string
	if d.Config != nil {
		if d.Config.IsProduction() {
			gin.SetMode(gin.ReleaseMode)
		}
		origins = d.Config.CORSOrigins
	}
	r := gin.New()
	h := handler.New(d.Services)
	healthHandler := health.New(d.DB)

	r.Use(
		middleware.RequestID(),
		middleware.Logger(log),
		middleware.Recovery(log),
		middleware.CORS(origins),
		middleware.BodyLimit(1<<20),
	)
	r.GET("/health", healthHandler.Handle)

	v1 := r.Group("/api/v1")
	{
		v1.POST("/auth/register", h.Register)
		v1.POST("/auth/login", h.Login)
		v1.POST("/auth/refresh", h.Refresh)
		v1.POST("/auth/logout", h.Logout)

		authed := v1.Group("", middleware.Auth(d.Tokens, d.Services))
		{
			authed.GET("/auth/me", h.Me)
			authed.PATCH("/auth/me", h.UpdateProfile)
			authed.POST("/auth/me/password", h.ChangePassword)

			authed.POST("/bookings", h.CreateBooking)
			authed.GET("/bookings", h.ListBookings)
			authed.GET("/bookings/:id", h.GetBooking)
			authed.PATCH("/bookings/:id/cancel", h.CancelBooking)

			authed.POST("/payments", h.CreatePayment)
			authed.GET("/payments/:id", h.GetPayment)

			authed.POST("/events/:id/register", h.RegisterForEvent)
			authed.DELETE("/events/:id/register", h.CancelEventRegistration)
			authed.GET("/events/registrations/mine", h.ListMyRegistrations)
		}

		optional := v1.Group("", middleware.OptionalAuth(d.Tokens, d.Services))
		{
			optional.GET("/sports", h.ListSports)
			optional.GET("/sports/:id", h.GetSport)
			optional.GET("/facilities", h.ListFacilities)
			optional.GET("/facilities/:id", h.GetFacility)
			optional.GET("/availability", h.GetAvailability)
			optional.GET("/events", h.ListEvents)
			optional.GET("/events/:id", h.GetEvent)
		}

		admin := v1.Group("/admin", middleware.Auth(d.Tokens, d.Services))
		{
			admin.GET("/dashboard", h.GetDashboard)

			admin.GET("/sports", h.ListAdminSports)
			admin.POST("/sports", h.CreateSport)
			admin.PATCH("/sports/:id", h.UpdateSport)
			admin.DELETE("/sports/:id", h.DeleteSport)

			admin.GET("/facilities", h.ListAdminFacilities)
			admin.POST("/facilities", h.CreateFacility)
			admin.PATCH("/facilities/:id", h.UpdateFacility)
			admin.DELETE("/facilities/:id", h.DeleteFacility)

			admin.GET("/facilities/:id/pricing-rules", h.ListPricingRules)
			admin.POST("/facilities/:id/pricing-rules", h.CreatePricingRule)
			admin.PATCH("/facilities/:id/pricing-rules/:ruleId", h.UpdatePricingRule)
			admin.DELETE("/facilities/:id/pricing-rules/:ruleId", h.DeletePricingRule)

			admin.GET("/facilities/:id/closures", h.ListClosures)
			admin.POST("/facilities/:id/closures", h.CreateClosure)
			admin.DELETE("/facilities/:id/closures/:closureId", h.DeleteClosure)

			admin.GET("/bookings", h.ListAdminBookings)
			admin.PATCH("/bookings/:id/status", h.UpdateBookingStatus)

			admin.GET("/payments", h.ListAdminPayments)
			admin.PATCH("/payments/:id", h.SettlePayment)

			admin.GET("/events", h.ListAdminEvents)
			admin.POST("/events", h.CreateEvent)
			admin.PATCH("/events/:id", h.UpdateEvent)
			admin.DELETE("/events/:id", h.DeleteEvent)
			admin.GET("/events/:id/registrations", h.ListEventRegistrations)

			staff := admin.Group("", middleware.RequireRoles(model.RoleStaff, model.RoleAdmin))
			{
				staff.GET("/users", h.ListUsers)
				staff.GET("/users/:id", h.GetUser)
				staff.GET("/audit-logs", h.ListAuditLogs)
			}

			adminOnly := admin.Group("", middleware.RequireRoles(model.RoleAdmin))
			{
				adminOnly.PATCH("/users/:id", h.UpdateUser)
				adminOnly.DELETE("/users/:id", h.DeleteUser)
			}
		}
	}

	return r
}
