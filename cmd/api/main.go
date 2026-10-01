// Command api boots the GOR booking HTTP server: configuration, logging,
// database, migrations, seed, services, router, scheduler and graceful
// shutdown.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/audit"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/auth"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/config"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/database"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/logger"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/repo"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/server"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/service"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	log := logger.New(cfg.LogLevel, cfg.AppEnv)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := database.Connect(ctx, cfg, log)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer db.Close()

	if cfg.MigrateOnStart {
		if err := db.Migrate(ctx, log); err != nil {
			return fmt.Errorf("apply migrations: %w", err)
		}
	}
	if cfg.SeedOnStart {
		if err := ensureAdmin(ctx, cfg, db, log); err != nil {
			return fmt.Errorf("seed admin: %w", err)
		}
	}

	tokens := auth.NewManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)
	auditor := audit.NewRecorder(db, log)
	svc := service.New(service.Deps{
		DB:     db,
		Cfg:    cfg,
		Log:    log,
		Audit:  auditor,
		Tokens: tokens,

		Users:         repo.NewUserRepo(db),
		RefreshTokens: repo.NewRefreshTokenRepo(db),
		Sports:        repo.NewSportRepo(db),
		Facilities:    repo.NewFacilityRepo(db),
		PricingRules:  repo.NewPricingRuleRepo(db),
		Closures:      repo.NewClosureRepo(db),
		Bookings:      repo.NewBookingRepo(db),
		Payments:      repo.NewPaymentRepo(db),
		Events:        repo.NewEventRepo(db),
		Dashboard:     repo.NewDashboardRepo(db),
		Audits:        repo.NewAuditRepo(db),
	})

	go svc.Scheduler.Run(ctx)

	engine := server.New(server.Deps{
		Config:   cfg,
		Log:      log,
		DB:       db,
		Tokens:   tokens,
		Services: svc,
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		log.Info("listening", "port", cfg.Port, "env", cfg.AppEnv)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case <-ctx.Done():
	case err := <-serveErr:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", "error", err)
		return err
	}
	if err := <-serveErr; err != nil {
		return err
	}
	log.Info("server stopped")
	return nil
}

// ensureAdmin creates the bootstrap administrator when seeding is enabled.
// The reference catalogue rows ship inside migration 0002_seed.sql, so only
// the admin account needs explicit code here.
func ensureAdmin(ctx context.Context, cfg *config.Config, db *database.DB, log *slog.Logger) error {
	hash, err := auth.HashPassword(cfg.AdminPassword)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}
	created, err := repo.NewUserRepo(db).EnsureAdmin(ctx, cfg.AdminEmail, cfg.AdminName, hash)
	if err != nil {
		return err
	}
	if created {
		log.Info("seeded admin account", "email", cfg.AdminEmail)
	}
	return nil
}
