// Package auth implements password hashing and JWT access/refresh tokens.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/apperr"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
)

// TokenType distinguishes access tokens from refresh tokens.
type TokenType string

// Supported token types.
const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

// Claims is the JWT payload issued by this service.
type Claims struct {
	UserID    uuid.UUID `json:"uid"`
	Role      model.Role `json:"role"`
	Email     string    `json:"email"`
	TokenType TokenType `json:"typ"`
	jwt.RegisteredClaims
}

// Manager issues and verifies JWTs.
type Manager struct {
	secret     []byte
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewManager builds a token manager.
func NewManager(secret, issuer string, accessTTL, refreshTTL time.Duration) *Manager {
	return &Manager{
		secret:     []byte(secret),
		issuer:     issuer,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// AccessTTL returns the configured access token lifetime.
func (m *Manager) AccessTTL() time.Duration { return m.accessTTL }

// RefreshTTL returns the configured refresh token lifetime.
func (m *Manager) RefreshTTL() time.Duration { return m.refreshTTL }

// IssueAccess signs a short lived access token for the user.
func (m *Manager) IssueAccess(u model.User) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(m.accessTTL)
	claims := Claims{
		UserID:    u.ID,
		Role:      u.Role,
		Email:     u.Email,
		TokenType: TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   u.ID.String(),
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return signed, expiresAt, nil
}

// ParseAccess verifies an access token and returns its claims.
func (m *Manager) ParseAccess(raw string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return m.secret, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, apperr.Unauthorized("invalid or expired access token")
	}
	if claims.TokenType != TokenTypeAccess {
		return nil, apperr.Unauthorized("invalid token type")
	}
	if claims.UserID == uuid.Nil {
		return nil, apperr.Unauthorized("invalid token subject")
	}
	return claims, nil
}

// GenerateRefreshToken returns a new opaque refresh token and its storage hash.
func GenerateRefreshToken() (raw string, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	return raw, HashToken(raw), nil
}

// HashToken returns the SHA-256 hex digest used to store opaque tokens.
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// bcryptCost is the work factor used for password hashing.
const bcryptCost = 12

// HashPassword hashes a plaintext password with bcrypt.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword compares a plaintext password against a bcrypt hash.
func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// dummyHash is compared against when a user does not exist, so that login
// latency does not reveal whether an email address is registered.
var dummyHash = "$2a$12$C6UzMDM.H6dfI/f/IKcEe.9SmcVTHn5cHqv3JPZ0Hf3F3Sd5nqWvK"

// VerifyPasswordConstantTime performs a bcrypt comparison even for missing users.
func VerifyPasswordConstantTime(hash, password string) bool {
	if hash == "" {
		hash = dummyHash
	}
	return VerifyPassword(hash, password)
}

type ctxKey string

const userCtxKey ctxKey = "auth.user"

// WithUser stores the authenticated user in the context.
func WithUser(ctx context.Context, u *model.User) context.Context {
	return context.WithValue(ctx, userCtxKey, u)
}

// UserFrom retrieves the authenticated user from the context.
func UserFrom(ctx context.Context) (*model.User, bool) {
	u, ok := ctx.Value(userCtxKey).(*model.User)
	return u, ok && u != nil
}

// MustUser returns the authenticated user or an unauthorized error.
func MustUser(ctx context.Context) (*model.User, error) {
	u, ok := UserFrom(ctx)
	if !ok {
		return nil, apperr.Unauthorized("authentication required")
	}
	return u, nil
}

// NormalizeEmail lowercases and trims an email address.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
