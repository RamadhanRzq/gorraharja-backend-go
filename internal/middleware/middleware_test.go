package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"booking-manager/internal/middleware"

	"github.com/gin-gonic/gin"
)

func setupAuthEngine(apiKey string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.APIKey(apiKey))
	r.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	r.GET("/api/v1/bookings", func(c *gin.Context) { c.JSON(200, gin.H{"data": []any{}}) })
	return r
}

func setupRBACEngine(adminKey, userKey string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Auth(adminKey, userKey))
	r.GET("/api/v1/bookings", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": []any{}, "role": middleware.GetRole(c)})
	})
	writes := r.Group("/api/v1/bookings", middleware.RequireAdmin())
	writes.PUT("/:id", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	writes.PATCH("/:id", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	writes.DELETE("/:id", func(c *gin.Context) { c.Status(204) })
	return r
}

func TestAPIKey(t *testing.T) {
	// Tanpa API_KEY: semua lolos (dev lokal).
	open := setupAuthEngine("")
	w := httptest.NewRecorder()
	open.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/bookings", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("tanpa API_KEY harus lolos, got %d", w.Code)
	}

	// Dengan API_KEY: /healthz publik, endpoint lain wajib kredensial.
	locked := setupAuthEngine("rahasia")
	w = httptest.NewRecorder()
	locked.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("healthz harus publik, got %d", w.Code)
	}

	w = httptest.NewRecorder()
	locked.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/bookings", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("tanpa kredensial harus 401, got %d", w.Code)
	}

	for _, h := range [][2]string{
		{"X-API-Key", "rahasia"},
		{"Authorization", "Bearer rahasia"},
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/bookings", nil)
		req.Header.Set(h[0], h[1])
		w = httptest.NewRecorder()
		locked.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s valid harus lolos, got %d", h[0], w.Code)
		}
	}
}

func TestAuth_Roles(t *testing.T) {
	r := setupRBACEngine("admin-123", "user-123")

	// Admin: baca lolos + tulis lolos.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bookings", nil)
	req.Header.Set("X-API-Key", "admin-123")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("admin GET harus lolos, got %d", w.Code)
	}
	for _, m := range []string{http.MethodPut, http.MethodPatch, http.MethodDelete} {
		req := httptest.NewRequest(m, "/api/v1/bookings/1", nil)
		req.Header.Set("Authorization", "Bearer admin-123")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if m == http.MethodDelete && w.Code != http.StatusNoContent {
			t.Fatalf("admin DELETE harus lolos, got %d", w.Code)
		} else if m != http.MethodDelete && w.Code != http.StatusOK {
			t.Fatalf("admin %s harus lolos, got %d", m, w.Code)
		}
	}

	// User: baca lolos + tulis ditolak 403.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/bookings", nil)
	req.Header.Set("Authorization", "Bearer user-123")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("user GET harus lolos, got %d", w.Code)
	}
	for _, m := range []string{http.MethodPut, http.MethodPatch, http.MethodDelete} {
		req := httptest.NewRequest(m, "/api/v1/bookings/1", nil)
		req.Header.Set("X-API-Key", "user-123")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("user %s harus 403, got %d", m, w.Code)
		}
	}

	// Kunci salah tetap 401.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/bookings", nil)
	req.Header.Set("X-API-Key", "salah")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("kunci salah harus 401, got %d", w.Code)
	}
}
