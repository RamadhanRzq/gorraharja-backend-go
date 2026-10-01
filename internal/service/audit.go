package service

import (
	"context"
	"strings"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/repo"
)

// AuditService exposes the audit trail to operators.
type AuditService struct {
	audits *repo.AuditRepo
}

// List returns audit entries, newest first, for staff and administrators.
func (s *AuditService) List(ctx context.Context, actor *model.User, f repo.AuditFilter, page, perPage, offset int) (model.Page[model.AuditLog], error) {
	if err := requireStaff(actor); err != nil {
		return model.Page[model.AuditLog]{}, err
	}
	f.Action = strings.TrimSpace(f.Action)
	f.EntityType = strings.TrimSpace(f.EntityType)
	f.Limit, f.Offset = perPage, offset
	items, total, err := s.audits.List(ctx, f)
	if err != nil {
		return model.Page[model.AuditLog]{}, err
	}
	return model.NewPage(items, page, perPage, total), nil
}
