package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CORS mengizinkan akses lintas origin (dikonfigurasi longgar untuk MVP).
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-API-Key, X-Request-ID")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// RequestID menyuntik X-Request-ID bila belum ada.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
		}
		c.Set("request_id", id)
		c.Writer.Header().Set("X-Request-ID", id)
		c.Next()
	}
}

// Role pengguna dari kredensial yang cocok.
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

const roleKey = "role"

// GetRole mengembalikan peran hasil autentikasi ("admin"/"user", "" bila tak ada).
func GetRole(c *gin.Context) string {
	if v, ok := c.Get(roleKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func bearerOrKey(c *gin.Context) string {
	if got := c.GetHeader("X-API-Key"); got != "" {
		return got
	}
	if auth := c.GetHeader("Authorization"); len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}
	return ""
}

func unauthorized(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"error": gin.H{
			"code":    "UNAUTHORIZED",
			"message": "Kredensial tidak valid atau tidak ada",
		},
	})
}

// Auth memvalidasi API key ganda dan menyuntik peran ke konteks.
// adminKey cocok -> peran "admin"; userKey cocok -> peran "user".
// Bila keduanya kosong (dev lokal), auth dilewati dengan peran admin.
func Auth(adminKey, userKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.FullPath() == "/healthz" || c.Request.URL.Path == "/healthz" {
			c.Next()
			return
		}
		if adminKey == "" && userKey == "" {
			c.Set(roleKey, RoleAdmin)
			c.Next()
			return
		}
		got := bearerOrKey(c)
		if got != "" {
			if adminKey != "" && got == adminKey {
				c.Set(roleKey, RoleAdmin)
				c.Next()
				return
			}
			if userKey != "" && got == userKey {
				c.Set(roleKey, RoleUser)
				c.Next()
				return
			}
		}
		unauthorized(c)
	}
}

// RequireAdmin menolak peran non-admin (user) dengan 403.
// Dipakai untuk PUT/PATCH/DELETE bookings: user read-only + create.
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if GetRole(c) != RoleAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"code":    "FORBIDDEN",
					"message": "Hanya admin yang boleh mengubah atau menghapus booking",
				},
			})
			return
		}
		c.Next()
	}
}

// APIKey menolak request tanpa kredensial (kecuali /healthz).
// Per PRD §9: API key via header (X-API-Key atau Authorization: Bearer).
// Bila apiKey kosong (dev lokal tanpa API_KEY), auth dilewati.
// Kompatibilitas legacy: satu kunci = peran admin. Kode baru pakai Auth.
func APIKey(apiKey string) gin.HandlerFunc {
	return Auth(apiKey, "")
}
