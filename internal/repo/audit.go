package repo

import (
	"context"

	"github.com/google/uuid"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/database"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
)

const auditColumns = `a.id, a.actor_id, a.actor_type, a.action, a.entity_type,
	a.entity_id, a.metadata, a.ip_address, a.created_at`

// AuditRepo reads the audit trail.
type AuditRepo struct {
	db *database.DB
}

// NewAuditRepo builds an audit repository.
func NewAuditRepo(db *database.DB) *AuditRepo { return &AuditRepo{db: db} }

// AuditFilter narrows an audit listing.
type AuditFilter struct {
	ActorID    *uuid.UUID
	Action     string
	EntityType string
	EntityID   *uuid.UUID
	Limit      int
	Offset     int
}

// List returns audit entries, newest first, with the total match count.
func (r *AuditRepo) List(ctx context.Context, f AuditFilter) ([]model.AuditLog, int, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT `+auditColumns+`, count(*) OVER () AS total
		FROM audit_logs a
		WHERE ($1::uuid IS NULL OR a.actor_id = $1)
		  AND ($2::text = '' OR a.action = $2)
		  AND ($3::text = '' OR a.entity_type = $3)
		  AND ($4::uuid IS NULL OR a.entity_id = $4)
		ORDER BY a.created_at DESC
		LIMIT $5 OFFSET $6`,
		f.ActorID, f.Action, f.EntityType, f.EntityID, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var (
		entries []model.AuditLog
		total   int
	)
	for rows.Next() {
		var a model.AuditLog
		if err := rows.Scan(&a.ID, &a.ActorID, &a.ActorType, &a.Action, &a.EntityType,
			&a.EntityID, &a.Metadata, &a.IPAddress, &a.CreatedAt, &total); err != nil {
			return nil, 0, err
		}
		entries = append(entries, a)
	}
	return entries, total, rows.Err()
}
