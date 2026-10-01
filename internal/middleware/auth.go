package middleware

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/apperr"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/auth"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/httpx"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
)

// UserLoader resolves the persisted user behind an access token.
type UserLoader interface {
	LoadUser(ctx context.Context, id string) (*model.User, error)
}

// TokenParser verifies access tokens.
type TokenParser interface {
	ParseAccess(raw string) (*auth.Claims, error)
}

// Auth authenticates requests carrying a Bearer access token and stores the
// persisted user in the request context.
func Auth(tokens TokenParser, users UserLoader) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := bearerToken(c)
		if raw == "" {
			httpx.Fail(c, apperr.Unauthorized("authorization header with bearer token is required"))
			return
		}
		claims, err := tokens.ParseAccess(raw)
		if err != nil {
			httpx.Fail(c, err)
			return
		}
		user, err := users.LoadUser(c.Request.Context(), claims.UserID.String())
		if err != nil {
			httpx.Fail(c, apperr.Unauthorized("account is no longer valid"))
			return
		}
		if !user.IsActive() {
			httpx.Fail(c, apperr.Forbidden("account is suspended"))
			return
		}
		c.Set(string(userContextKey), user)
		c.Next()
	}
}

// OptionalAuth attaches the user when a valid token is present, otherwise it
// continues anonymously. Invalid tokens are ignored.
func OptionalAuth(tokens TokenParser, users UserLoader) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := bearerToken(c)
		if raw == "" {
			c.Next()
			return
		}
		claims, err := tokens.ParseAccess(raw)
		if err != nil {
			c.Next()
			return
		}
		user, err := users.LoadUser(c.Request.Context(), claims.UserID.String())
		if err != nil || !user.IsActive() {
			c.Next()
			return
		}
		c.Set(string(userContextKey), user)
		c.Next()
	}
}

// RequireRoles restricts a route to the listed roles.
func RequireRoles(roles ...model.Role) gin.HandlerFunc {
	allowed := make(map[model.Role]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(c *gin.Context) {
		user, err := User(c)
		if err != nil {
			httpx.Fail(c, err)
			return
		}
		if _, ok := allowed[user.Role]; !ok {
			httpx.Fail(c, apperr.Forbidden("you do not have permission to perform this action"))
			return
		}
		c.Next()
	}
}

type ctxKey string

const userContextKey ctxKey = "auth.user"

// User returns the authenticated user attached by Auth or OptionalAuth.
func User(c *gin.Context) (*model.User, error) {
	v, ok := c.Get(string(userContextKey))
	if !ok {
		return nil, apperr.Unauthorized("authentication required")
	}
	user, ok := v.(*model.User)
	if !ok || user == nil {
		return nil, apperr.Unauthorized("authentication required")
	}
	return user, nil
}

// UserOrNil returns the authenticated user when present.
func UserOrNil(c *gin.Context) *model.User {
	user, err := User(c)
	if err != nil {
		return nil
	}
	return user
}

func bearerToken(c *gin.Context) string {
	header := strings.TrimSpace(c.GetHeader("Authorization"))
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
