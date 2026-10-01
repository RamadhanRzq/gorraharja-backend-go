package service

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/apperr"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/audit"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/auth"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/config"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/database"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/repo"
)

// phonePattern accepts international and local Indonesian phone numbers.
var phonePattern = regexp.MustCompile(`^\+?[0-9][0-9\s\-()]{6,19}$`)

// AuthService registers accounts and issues access/refresh token pairs.
type AuthService struct {
	db      *database.DB
	cfg     *config.Config
	log     *slog.Logger
	audit   *audit.Recorder
	tokens  *auth.Manager
	users   *repo.UserRepo
	refresh *repo.RefreshTokenRepo
}

// RegisterInput is the sign-up payload of a customer account.
type RegisterInput struct {
	Email    string `json:"email" validate:"required,email,max=254"`
	FullName string `json:"full_name" validate:"required,min=2,max=120"`
	Phone    string `json:"phone" validate:"omitempty,max=32"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

// LoginInput is the credential payload.
type LoginInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// RefreshInput carries the opaque refresh token to rotate.
type RefreshInput struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// ProfileUpdateInput is the partial update payload of the caller's profile.
type ProfileUpdateInput struct {
	FullName *string `json:"full_name" validate:"omitempty,min=2,max=120"`
	Phone    *string `json:"phone" validate:"omitempty,max=32"`
}

// PasswordChangeInput replaces the caller's password.
type PasswordChangeInput struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8,max=72"`
}

// AuthResult is the token pair returned by register/login/refresh.
type AuthResult struct {
	User         *model.User `json:"user"`
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	TokenType    string      `json:"token_type"`
	ExpiresIn    int         `json:"expires_in"`
	ExpiresAt    time.Time   `json:"expires_at"`
}

// Register creates a customer account and returns an initial token pair.
func (s *AuthService) Register(ctx context.Context, in RegisterInput, meta RequestMeta) (*AuthResult, error) {
	email := auth.NormalizeEmail(in.Email)
	if !strings.Contains(email, "@") {
		return nil, apperr.BadRequest("invalid email address")
	}
	phone, err := normalizePhone(in.Phone)
	if err != nil {
		return nil, err
	}
	if _, err := s.users.FindByEmail(ctx, email); err == nil {
		return nil, apperr.Conflict("an account with this email already exists")
	} else if !apperr.Is(err, apperr.CodeNotFound) {
		return nil, err
	}

	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	user, err := s.users.Create(ctx, email, strings.TrimSpace(in.FullName), phone, hash, model.RoleCustomer)
	if err != nil {
		if uniqueViolation(err, "users_email_active_key") {
			return nil, apperr.Conflict("an account with this email already exists")
		}
		return nil, err
	}
	s.audit.RecordDetached(ctx, audit.Entry{
		ActorID: &user.ID, Action: "auth.register", EntityType: "user", EntityID: &user.ID,
		Metadata: model.JSONMap{"email": user.Email}, IPAddress: meta.IP,
	})
	return s.issue(ctx, user, meta)
}

// Login verifies credentials and issues a token pair.
func (s *AuthService) Login(ctx context.Context, in LoginInput, meta RequestMeta) (*AuthResult, error) {
	email := auth.NormalizeEmail(in.Email)
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		if apperr.Is(err, apperr.CodeNotFound) {
			// Keep the timing of a failed lookup indistinguishable from a bad password.
			auth.VerifyPasswordConstantTime("", in.Password)
			s.audit.RecordDetached(ctx, audit.Entry{
				Action: "auth.login_failed", EntityType: "user",
				Metadata: model.JSONMap{"email": email}, IPAddress: meta.IP,
			})
			return nil, apperr.Unauthorized("invalid email or password")
		}
		return nil, err
	}
	if !auth.VerifyPassword(user.PasswordHash, in.Password) {
		s.audit.RecordDetached(ctx, audit.Entry{
			ActorID: &user.ID, Action: "auth.login_failed", EntityType: "user", EntityID: &user.ID,
			Metadata: model.JSONMap{"email": user.Email}, IPAddress: meta.IP,
		})
		return nil, apperr.Unauthorized("invalid email or password")
	}
	if !user.IsActive() {
		return nil, apperr.Forbidden("account is suspended")
	}
	s.audit.RecordDetached(ctx, audit.Entry{
		ActorID: &user.ID, Action: "auth.login", EntityType: "user", EntityID: &user.ID,
		Metadata: model.JSONMap{"email": user.Email}, IPAddress: meta.IP,
	})
	return s.issue(ctx, user, meta)
}

// Refresh rotates a refresh token: the presented token is revoked and a new
// pair is issued, so a stolen token can be used at most once.
func (s *AuthService) Refresh(ctx context.Context, in RefreshInput, meta RequestMeta) (*AuthResult, error) {
	token, err := s.refresh.FindByHash(ctx, auth.HashToken(strings.TrimSpace(in.RefreshToken)))
	if err != nil {
		if apperr.Is(err, apperr.CodeNotFound) {
			return nil, apperr.Unauthorized("invalid refresh token")
		}
		return nil, err
	}
	if !token.Active(time.Now()) {
		return nil, apperr.Unauthorized("refresh token is expired or revoked")
	}
	user, err := s.users.FindByID(ctx, token.UserID)
	if err != nil || !user.IsActive() {
		return nil, apperr.Unauthorized("account is no longer valid")
	}
	if err := s.refresh.Revoke(ctx, token.ID); err != nil {
		return nil, err
	}
	return s.issue(ctx, user, meta)
}

// Logout revokes the presented refresh token. Unknown tokens are ignored so
// that signing out is idempotent.
func (s *AuthService) Logout(ctx context.Context, refreshToken string, meta RequestMeta) error {
	raw := strings.TrimSpace(refreshToken)
	if raw == "" {
		return nil
	}
	token, err := s.refresh.FindByHash(ctx, auth.HashToken(raw))
	if err != nil {
		if apperr.Is(err, apperr.CodeNotFound) {
			return nil
		}
		return err
	}
	if err := s.refresh.Revoke(ctx, token.ID); err != nil {
		return err
	}
	s.audit.RecordDetached(ctx, audit.Entry{
		ActorID: &token.UserID, Action: "auth.logout", EntityType: "user", EntityID: &token.UserID,
		IPAddress: meta.IP,
	})
	return nil
}

// Me returns the authenticated account.
func (s *AuthService) Me(ctx context.Context, actor *model.User) (*model.User, error) {
	if err := requireActor(actor); err != nil {
		return nil, err
	}
	return actor, nil
}

// UpdateProfile changes the mutable profile fields of the caller.
func (s *AuthService) UpdateProfile(ctx context.Context, actor *model.User, in ProfileUpdateInput, meta RequestMeta) (*model.User, error) {
	if err := requireActor(actor); err != nil {
		return nil, err
	}
	fullName := actor.FullName
	if in.FullName != nil {
		fullName = strings.TrimSpace(*in.FullName)
		if fullName == "" {
			return nil, apperr.BadRequest("invalid profile", map[string]string{"full_name": "is required"})
		}
	}
	phone := ""
	if actor.Phone != nil {
		phone = *actor.Phone
	}
	if in.Phone != nil {
		normalized, err := normalizePhone(*in.Phone)
		if err != nil {
			return nil, err
		}
		phone = normalized
	}
	user, err := s.users.UpdateProfile(ctx, actor.ID, fullName, phone)
	if err != nil {
		return nil, err
	}
	s.audit.RecordDetached(ctx, audit.Entry{
		ActorID: &user.ID, Action: "auth.profile_update", EntityType: "user", EntityID: &user.ID,
		Metadata: model.JSONMap{"email": user.Email}, IPAddress: meta.IP,
	})
	return user, nil
}

// ChangePassword replaces the caller's password and revokes every session.
func (s *AuthService) ChangePassword(ctx context.Context, actor *model.User, in PasswordChangeInput, meta RequestMeta) error {
	if err := requireActor(actor); err != nil {
		return err
	}
	if !auth.VerifyPassword(actor.PasswordHash, in.CurrentPassword) {
		return apperr.Unauthorized("current password is incorrect")
	}
	if in.CurrentPassword == in.NewPassword {
		return apperr.BadRequest("new password must differ from the current password")
	}
	hash, err := auth.HashPassword(in.NewPassword)
	if err != nil {
		return apperr.Internal(err)
	}
	if err := s.users.UpdatePassword(ctx, actor.ID, hash); err != nil {
		return err
	}
	if err := s.refresh.RevokeAllForUser(ctx, actor.ID); err != nil {
		return err
	}
	s.audit.RecordDetached(ctx, audit.Entry{
		ActorID: &actor.ID, Action: "auth.password_change", EntityType: "user", EntityID: &actor.ID,
		IPAddress: meta.IP,
	})
	return nil
}

// issue signs an access token and stores the hash of a fresh refresh token.
func (s *AuthService) issue(ctx context.Context, user *model.User, meta RequestMeta) (*AuthResult, error) {
	access, expiresAt, err := s.tokens.IssueAccess(*user)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	raw, hash, err := auth.GenerateRefreshToken()
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if _, err := s.refresh.Create(ctx, user.ID, hash, time.Now().Add(s.tokens.RefreshTTL()),
		meta.UserAgent, meta.IP); err != nil {
		return nil, err
	}
	return &AuthResult{
		User:         user,
		AccessToken:  access,
		RefreshToken: raw,
		TokenType:    "Bearer",
		ExpiresIn:    int(time.Until(expiresAt).Seconds()),
		ExpiresAt:    expiresAt,
	}, nil
}

// normalizePhone trims and validates an optional phone number.
func normalizePhone(raw string) (string, error) {
	phone := strings.TrimSpace(raw)
	if phone == "" {
		return "", nil
	}
	if !phonePattern.MatchString(phone) {
		return "", apperr.BadRequest("invalid phone number", map[string]string{
			"phone": "must be 7-20 characters of digits, spaces, parentheses, '-' or a leading '+'",
		})
	}
	return phone, nil
}
