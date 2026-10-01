package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/config"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/repo"
)

// Scheduler runs the background maintenance sweeps: expiring unpaid bookings,
// completing finished ones and pruning dead refresh tokens.
type Scheduler struct {
	cfg      *config.Config
	log      *slog.Logger
	bookings *repo.BookingRepo
	refresh  *repo.RefreshTokenRepo
}

// Run starts the sweep loop and blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	interval := s.cfg.ExpirySweepInterval
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	s.log.Info("scheduler started", "interval", interval.String())
	s.SweepOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			s.log.Info("scheduler stopped")
			return
		case <-ticker.C:
			s.SweepOnce(ctx)
		}
	}
}

// SweepOnce performs a single maintenance pass.
func (s *Scheduler) SweepOnce(ctx context.Context) {
	now := time.Now()
	if n, err := s.bookings.ExpirePending(ctx, now); err != nil {
		s.log.Error("failed to expire pending bookings", "error", err)
	} else if n > 0 {
		s.log.Info("expired pending bookings", "count", n)
	}
	if n, err := s.bookings.CompleteFinished(ctx, now); err != nil {
		s.log.Error("failed to complete finished bookings", "error", err)
	} else if n > 0 {
		s.log.Info("completed finished bookings", "count", n)
	}
	if n, err := s.refresh.DeleteExpired(ctx, now); err != nil {
		s.log.Error("failed to prune refresh tokens", "error", err)
	} else if n > 0 {
		s.log.Info("pruned refresh tokens", "count", n)
	}
}
