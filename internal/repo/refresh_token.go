package repo

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/database"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
)

// RefreshTokenRepo persists rotating refresh tokens.
type RefreshTokenRepo struct {
	db *database.DB
}

// NewRefreshTokenRepo builds a refresh token repository.
func NewRefreshTokenRepo(db *database.DB) *RefreshTokenRepo { return &RefreshTokenRepo{db: db} }

// Create stores a new refresh token hash for a user.
func (r *RefreshTokenRepo) Create(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time, userAgent, ip string) (*model.RefreshToken, error) {
	var t model.RefreshToken
	err := r.db.Pool().QueryRow(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at, user_agent, ip_address)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, token_hash, expires_at, revoked_at, user_agent, ip_address, created_at`,
		userID, tokenHash, expiresAt, userAgent, ip).
		Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.RevokedAt, &t.UserAgent, &t.IPAddress, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// FindByHash returns the token row matching a stored hash.
func (r *RefreshTokenRepo) FindByHash(ctx context.Context, tokenHash string) (*model.RefreshToken, error) {
	var t model.RefreshToken
	err := r.db.Pool().QueryRow(ctx, `
		SELECT id, user_id, token_hash, expires_at, revoked_at, user_agent, ip_address, created_at
		FROM refresh_tokens WHERE token_hash = $1`, tokenHash).
		Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.RevokedAt, &t.UserAgent, &t.IPAddress, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// Revoke marks a single token as revoked. Already revoked tokens are ignored.
func (r *RefreshTokenRepo) Revoke(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Pool().Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`, id)
	return err
}

// RevokeAllForUser revokes every active refresh token of a user.
func (r *RefreshTokenRepo) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Pool().Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

// DeleteExpired removes tokens that expired before the given time.
func (r *RefreshTokenRepo) DeleteExpired(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.db.Pool().Exec(ctx,
		`DELETE FROM refresh_tokens WHERE expires_at < $1 OR revoked_at IS NOT NULL AND revoked_at < $1`,
		before)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
