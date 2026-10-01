// Package audit records state changing actions for traceability.
package audit

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/database"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
)

// Entry describes a single audited action.
type Entry struct {
	ActorID    *uuid.UUID
	ActorType  model.ActorType
	Action     string
	EntityType string
	EntityID   *uuid.UUID
	Metadata   model.JSONMap
	IPAddress  string
}

// Recorder writes audit log rows.
type Recorder struct {
	db  *database.DB
	log *slog.Logger
}

// NewRecorder builds an audit recorder.
func NewRecorder(db *database.DB, log *slog.Logger) *Recorder {
	return &Recorder{db: db, log: log}
}

// Record writes the entry inside the caller's transaction, so that the audit
// trail and the mutation commit or roll back together.
func (r *Recorder) Record(ctx context.Context, tx pgx.Tx, e Entry) error {
	if e.ActorType == "" {
		e.ActorType = model.ActorUser
	}
	if e.Metadata == nil {
		e.Metadata = model.JSONMap{}
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO audit_logs (actor_id, actor_type, action, entity_type, entity_id, metadata, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		e.ActorID, e.ActorType, e.Action, e.EntityType, e.EntityID, e.Metadata, e.IPAddress)
	return err
}

// RecordDetached writes the entry on its own connection and only logs failures.
// Use it for actions that must not fail the request (e.g. login attempts).
func (r *Recorder) RecordDetached(ctx context.Context, e Entry) {
	if e.ActorType == "" {
		e.ActorType = model.ActorUser
	}
	if e.Metadata == nil {
		e.Metadata = model.JSONMap{}
	}
	if _, err := r.db.Pool().Exec(ctx, `
		INSERT INTO audit_logs (actor_id, actor_type, action, entity_type, entity_id, metadata, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		e.ActorID, e.ActorType, e.Action, e.EntityType, e.EntityID, e.Metadata, e.IPAddress); err != nil {
		r.log.Error("failed to write audit log", "action", e.Action, "error", err)
	}
}
