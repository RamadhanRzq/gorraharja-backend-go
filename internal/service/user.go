package service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/apperr"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/audit"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/repo"
)

// UserService manages accounts for administrators.
type UserService struct {
	users   *repo.UserRepo
	refresh *repo.RefreshTokenRepo
	audit   *audit.Recorder
}

// UserAdminUpdateInput is the administrative update payload of an account.
type UserAdminUpdateInput struct {
	Role   *model.Role       `json:"role"`
	Status *model.UserStatus `json:"status"`
}

// List returns accounts for staff and administrators.
func (s *UserService) List(ctx context.Context, actor *model.User, f repo.ListFilter, page, perPage, offset int) (model.Page[model.User], error) {
	if err := requireStaff(actor); err != nil {
		return model.Page[model.User]{}, err
	}
	f.Search = strings.TrimSpace(f.Search)
	f.Limit, f.Offset = perPage, offset
	items, total, err := s.users.List(ctx, f)
	if err != nil {
		return model.Page[model.User]{}, err
	}
	return model.NewPage(items, page, perPage, total), nil
}

// Get returns a single account for staff and administrators.
func (s *UserService) Get(ctx context.Context, actor *model.User, id uuid.UUID) (*model.User, error) {
	if err := requireStaff(actor); err != nil {
		return nil, err
	}
	return s.users.FindByID(ctx, id)
}

// Update applies an administrative role/status change. Demoting or suspending
// the last remaining admin is refused so the venue cannot lock itself out.
func (s *UserService) Update(ctx context.Context, actor *model.User, id uuid.UUID, in UserAdminUpdateInput, meta RequestMeta) (*model.User, error) {
	if err := requireAdmin(actor); err != nil {
		return nil, err
	}
	if in.Role != nil && !in.Role.Valid() {
		return nil, apperr.BadRequest("invalid role")
	}
	if in.Status != nil && *in.Status != model.UserStatusActive && *in.Status != model.UserStatusSuspended {
		return nil, apperr.BadRequest("invalid user status")
	}
	target, err := s.users.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if demotesAdmin(target, in) || suspendsAccount(target, in) {
		last, err := s.lastAdmin(ctx, target.ID)
		if err != nil {
			return nil, err
		}
		if last {
			return nil, apperr.Conflict("cannot remove the last administrator account")
		}
	}
	updated, err := s.users.AdminUpdate(ctx, id, in.Role, in.Status)
	if err != nil {
		return nil, err
	}
	s.audit.RecordDetached(ctx, audit.Entry{
		ActorID: &actor.ID, Action: "user.update", EntityType: "user", EntityID: &updated.ID,
		Metadata: model.JSONMap{"email": updated.Email, "role": string(updated.Role),
			"status": string(updated.Status)},
		IPAddress: meta.IP,
	})
	return updated, nil
}

// Delete soft-deletes an account and revokes its sessions. Deleting the last
// admin or one's own account is refused.
func (s *UserService) Delete(ctx context.Context, actor *model.User, id uuid.UUID, meta RequestMeta) error {
	if err := requireAdmin(actor); err != nil {
		return err
	}
	if actor.ID == id {
		return apperr.Conflict("you cannot delete your own account")
	}
	target, err := s.users.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if target.Role == model.RoleAdmin {
		last, err := s.lastAdmin(ctx, target.ID)
		if err != nil {
			return err
		}
		if last {
			return apperr.Conflict("cannot delete the last administrator account")
		}
	}
	if err := s.users.SoftDelete(ctx, id); err != nil {
		return err
	}
	if err := s.refresh.RevokeAllForUser(ctx, id); err != nil {
		return err
	}
	s.audit.RecordDetached(ctx, audit.Entry{
		ActorID: &actor.ID, Action: "user.delete", EntityType: "user", EntityID: &id,
		Metadata:  model.JSONMap{"email": target.Email},
		IPAddress: meta.IP,
	})
	return nil
}

// lastAdmin reports whether id is the only active administrator.
func (s *UserService) lastAdmin(ctx context.Context, id uuid.UUID) (bool, error) {
	n, err := s.users.CountAdmins(ctx)
	if err != nil {
		return false, err
	}
	if n > 1 {
		return false, nil
	}
	sole, err := s.users.FindByID(ctx, id)
	if err != nil {
		return false, err
	}
	return sole.Role == model.RoleAdmin, nil
}

func demotesAdmin(target *model.User, in UserAdminUpdateInput) bool {
	return target.Role == model.RoleAdmin && in.Role != nil && *in.Role != model.RoleAdmin
}

func suspendsAccount(target *model.User, in UserAdminUpdateInput) bool {
	return target.Role == model.RoleAdmin && in.Status != nil &&
		*in.Status != model.UserStatusActive && target.Status == model.UserStatusActive
}
