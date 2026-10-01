// Package middleware provides Gin middleware for logging, recovery, CORS and auth.
package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/apperr"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/httpx"
)

// HeaderRequestID is the header carrying the request correlation id.
const HeaderRequestID = "X-Request-ID"

const requestIDKey = "request_id"

// RequestID assigns or propagates a correlation id for every request.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.GetHeader(HeaderRequestID))
		if id == "" {
			id = uuid.NewString()
		}
		c.Set(requestIDKey, id)
		c.Header(HeaderRequestID, id)
		c.Next()
	}
}

// RequestIDFrom returns the correlation id of the current request.
func RequestIDFrom(c *gin.Context) string {
	if v, ok := c.Get(requestIDKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// ClientIP returns the best-effort client address of the request.
func ClientIP(c *gin.Context) string {
	return c.ClientIP()
}

// Logger emits one structured access log line per request.
func Logger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		c.Next()

		status := c.Writer.Status()
		attrs := []any{
			"method", c.Request.Method,
			"path", path,
			"status", status,
			"duration_ms", float64(time.Since(start).Microseconds()) / 1000.0,
			"ip", c.ClientIP(),
			"request_id", RequestIDFrom(c),
			"size", c.Writer.Size(),
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, "error", c.Errors.String())
		}

		switch {
		case status >= http.StatusInternalServerError:
			log.Error("request completed", attrs...)
		case status >= http.StatusBadRequest:
			log.Warn("request completed", attrs...)
		default:
			log.Info("request completed", attrs...)
		}
	}
}

// Recovery converts panics into a 500 response without leaking internals.
func Recovery(log *slog.Logger) gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, recovered any) {
		log.Error("panic recovered",
			"error", recovered,
			"path", c.Request.URL.Path,
			"request_id", RequestIDFrom(c))
		httpx.Fail(c, apperr.New(http.StatusInternalServerError, apperr.CodeInternal, "internal server error"))
	})
}

// CORS applies an allow-list based CORS policy.
func CORS(origins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(origins))
	wildcard := false
	for _, o := range origins {
		o = strings.TrimSpace(strings.TrimSuffix(o, "/"))
		if o == "" {
			continue
		}
		if o == "*" {
			wildcard = true
		}
		allowed[o] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := strings.TrimSuffix(strings.TrimSpace(c.GetHeader("Origin")), "/")
		if origin != "" {
			_, ok := allowed[origin]
			if wildcard || ok {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Vary", "Origin")
				c.Header("Access-Control-Allow-Credentials", "true")
				c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
				c.Header("Access-Control-Allow-Headers",
					"Authorization, Content-Type, Accept, X-Request-ID")
				c.Header("Access-Control-Expose-Headers", HeaderRequestID)
				c.Header("Access-Control-Max-Age", "600")
			}
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// BodyLimit rejects oversized request bodies.
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}
