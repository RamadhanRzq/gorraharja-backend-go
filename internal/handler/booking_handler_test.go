package handler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"booking-manager/internal/database"
	"booking-manager/internal/handler"
	"booking-manager/internal/repository"
	"booking-manager/internal/router"
	"booking-manager/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestServer(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := repository.NewBookingRepository(db)
	svc := service.NewBookingService(repo)
	return router.New(handler.NewBookingHandler(svc), "", "")
}

func doRequest(t *testing.T, e *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf *bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		buf = bytes.NewBuffer(b)
	} else {
		buf = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	return w
}

func createBooking(t *testing.T, e *gin.Engine, tanggal, jam string, durasi int, nama string, nominal int64) int {
	t.Helper()
	w := doRequest(t, e, http.MethodPost, "/api/v1/bookings", map[string]any{
		"tanggal": tanggal, "jam": jam, "durasi": durasi,
		"nama_penyewa": nama, "nominal_pembayaran": nominal,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create: status %d body %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	return int(resp["data"].(map[string]any)["id"].(float64))
}

func TestCreate_OK(t *testing.T) {
	e := setupTestServer(t)
	w := doRequest(t, e, http.MethodPost, "/api/v1/bookings", map[string]any{
		"tanggal": "2026-10-10", "jam": "19:00", "durasi": 120,
		"nama_penyewa": "Budi Santoso", "nominal_pembayaran": 300000,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestCreate_AliasKeys(t *testing.T) {
	// Payload frontend: durasi_menit + nominal (bukan durasi/nominal_pembayaran).
	e := setupTestServer(t)
	w := doRequest(t, e, http.MethodPost, "/api/v1/bookings", map[string]any{
		"tanggal": "2026-10-03", "jam": "21:00", "durasi_menit": 120,
		"nama_penyewa": "Wungurejo", "nominal": 60000,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	if int(data["durasi"].(float64)) != 120 {
		t.Fatalf("durasi = %v, want 120", data["durasi"])
	}
	if int(data["nominal_pembayaran"].(float64)) != 60000 {
		t.Fatalf("nominal = %v, want 60000", data["nominal_pembayaran"])
	}
}

func TestCreate_IgnoresClientID(t *testing.T) {
	// Form kadang menyertakan id: "" saat insert; backend harus mengabaikannya.
	e := setupTestServer(t)
	w := doRequest(t, e, http.MethodPost, "/api/v1/bookings", map[string]any{
		"id": "", "tanggal": "2026-10-03", "jam": "21:00", "durasi": 120,
		"nama_penyewa": "Wungurejo", "nominal_pembayaran": 60000,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	if id, ok := data["id"].(float64); !ok || id < 1 {
		t.Fatalf("id respons harus auto-increment, got %v", data["id"])
	}
}

func TestCreate_ValidationError(t *testing.T) {
	e := setupTestServer(t)
	w := doRequest(t, e, http.MethodPost, "/api/v1/bookings", map[string]any{
		"tanggal": "2026-10-10", "jam": "25:00", "durasi": 0,
		"nama_penyewa": "", "nominal_pembayaran": -5,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", w.Code)
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	errObj := resp["error"].(map[string]any)
	if errObj["code"] != "VALIDATION_ERROR" {
		t.Fatalf("code = %v", errObj["code"])
	}
	if _, ok := errObj["details"]; !ok {
		t.Fatal("details wajib ada")
	}
}

func TestList_FilterSearchPagination(t *testing.T) {
	e := setupTestServer(t)
	createBooking(t, e, "2026-10-10", "19:00", 120, "Budi Santoso", 300000)
	createBooking(t, e, "2026-10-11", "08:00", 60, "Siti Aminah", 150000)
	createBooking(t, e, "2026-10-12", "10:00", 90, "budi kecil", 200000)

	// Search case-insensitive: "BUDI" harus kena 2.
	w := doRequest(t, e, http.MethodGet, "/api/v1/bookings?q=BUDI", nil)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	meta := resp["meta"].(map[string]any)
	if int(meta["total"].(float64)) != 2 {
		t.Fatalf("search total = %v, want 2", meta["total"])
	}
	if int(meta["total_nominal"].(float64)) != 500000 {
		t.Fatalf("total_nominal = %v, want 500000", meta["total_nominal"])
	}

	// Filter tanggal inklusif + pagination.
	w = doRequest(t, e, http.MethodGet,
		"/api/v1/bookings?start_date=2026-10-11&end_date=2026-10-12&page=1&limit=1", nil)
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	meta = resp["meta"].(map[string]any)
	if int(meta["total"].(float64)) != 2 {
		t.Fatalf("filter total = %v, want 2", meta["total"])
	}
	data := resp["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("limit=1 harus 1 baris, got %d", len(data))
	}
	// Urutan ASC: baris pertama = 2026-10-11.
	first := data[0].(map[string]any)
	if first["tanggal"] != "2026-10-11" {
		t.Fatalf("urutan salah: %v", first["tanggal"])
	}
}

func TestDetail_NotFound(t *testing.T) {
	e := setupTestServer(t)
	w := doRequest(t, e, http.MethodGet, "/api/v1/bookings/999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404", w.Code)
	}
}

func TestUpdateDelete_NotFound(t *testing.T) {
	e := setupTestServer(t)
	payload := map[string]any{
		"tanggal": "2026-10-10", "jam": "19:00", "durasi": 60,
		"nama_penyewa": "X", "nominal_pembayaran": 1000,
	}
	if w := doRequest(t, e, http.MethodPut, "/api/v1/bookings/999", payload); w.Code != http.StatusNotFound {
		t.Fatalf("PUT status %d, want 404", w.Code)
	}
	if w := doRequest(t, e, http.MethodDelete, "/api/v1/bookings/999", nil); w.Code != http.StatusNotFound {
		t.Fatalf("DELETE status %d, want 404", w.Code)
	}
}

func TestUpdatePatchDelete_Flow(t *testing.T) {
	e := setupTestServer(t)
	id := createBooking(t, e, "2026-10-10", "19:00", 120, "Budi", 300000)

	// PATCH sebagian.
	w := doRequest(t, e, http.MethodPatch, fmt.Sprintf("/api/v1/bookings/%d", id),
		map[string]any{"durasi": 60})
	if w.Code != http.StatusOK {
		t.Fatalf("PATCH status %d body %s", w.Code, w.Body.String())
	}

	// PUT penuh.
	w = doRequest(t, e, http.MethodPut, fmt.Sprintf("/api/v1/bookings/%d", id), map[string]any{
		"tanggal": "2026-10-15", "jam": "20:00", "durasi": 90,
		"nama_penyewa": "Budi Baru", "nominal_pembayaran": 250000,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("PUT status %d body %s", w.Code, w.Body.String())
	}

	// DELETE → 204, lalu GET → 404 (soft delete tersembunyi).
	if w := doRequest(t, e, http.MethodDelete, fmt.Sprintf("/api/v1/bookings/%d", id), nil); w.Code != http.StatusNoContent {
		t.Fatalf("DELETE status %d, want 204", w.Code)
	}
	if w := doRequest(t, e, http.MethodGet, fmt.Sprintf("/api/v1/bookings/%d", id), nil); w.Code != http.StatusNotFound {
		t.Fatalf("GET setelah delete status %d, want 404", w.Code)
	}
}

func TestExportPDF_Cutoff(t *testing.T) {
	e := setupTestServer(t)
	createBooking(t, e, "2026-10-10", "19:00", 120, "Budi", 300000)
	createBooking(t, e, "2026-10-20", "08:00", 60, "Siti", 150000)
	createBooking(t, e, "2026-11-05", "10:00", 60, "Andi", 999000) // di luar cutoff

	w := doRequest(t, e, http.MethodGet, "/api/v1/bookings/export/pdf?cutoff_date=2026-10-31&start_date=2026-10-01", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("export status %d body %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("content-type = %q", ct)
	}
	if cd := w.Header().Get("Content-Disposition"); cd == "" {
		t.Fatal("content-disposition wajib ada")
	}
	if !bytes.HasPrefix(w.Body.Bytes(), []byte("%PDF")) {
		t.Fatal("bukan PDF")
	}
}

func TestExportPDF_TanpaCutoff400(t *testing.T) {
	e := setupTestServer(t)
	w := doRequest(t, e, http.MethodGet, "/api/v1/bookings/export/pdf", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", w.Code)
	}
}

func TestHealthz(t *testing.T) {
	e := setupTestServer(t)
	w := doRequest(t, e, http.MethodGet, "/healthz", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", w.Code)
	}
}

func setupRBACServer(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := repository.NewBookingRepository(db)
	svc := service.NewBookingService(repo)
	return router.New(handler.NewBookingHandler(svc), "kunci-admin", "kunci-user")
}

func doAuthRequest(t *testing.T, e *gin.Engine, method, path, key string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf *bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		buf = bytes.NewBuffer(b)
	} else {
		buf = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	return w
}

func TestRBAC_MeRole(t *testing.T) {
	e := setupRBACServer(t)
	for _, tc := range []struct {
		key  string
		role string
	}{
		{"kunci-admin", "admin"},
		{"kunci-user", "user"},
	} {
		w := doAuthRequest(t, e, http.MethodGet, "/api/v1/me", tc.key, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("GET /me (%s): status %d body %s", tc.role, w.Code, w.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		data, ok := resp["data"].(map[string]any)
		if !ok || data["role"] != tc.role {
			t.Fatalf("GET /me (%s): body %s", tc.role, w.Body.String())
		}
	}
}

func TestRBAC_UserBolehBacaTambah_TolakUbahHapus(t *testing.T) {
	e := setupRBACServer(t)
	user, admin := "kunci-user", "kunci-admin"
	payload := map[string]any{
		"tanggal": "2026-10-10", "jam": "19:00", "durasi": 120,
		"nama_penyewa": "Sinta", "nominal_pembayaran": 300000,
	}

	// Tanpa kredensial: 401.
	if w := doAuthRequest(t, e, http.MethodGet, "/api/v1/bookings", "", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("tanpa kredensial status %d, want 401", w.Code)
	}

	// User boleh tambah (POST) lalu baca (GET).
	w := doAuthRequest(t, e, http.MethodPost, "/api/v1/bookings", user, payload)
	if w.Code != http.StatusCreated {
		t.Fatalf("user POST status %d body %s", w.Code, w.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	id := int(created["data"].(map[string]any)["id"].(float64))
	path := fmt.Sprintf("/api/v1/bookings/%d", id)
	if w := doAuthRequest(t, e, http.MethodGet, path, user, nil); w.Code != http.StatusOK {
		t.Fatalf("user GET status %d body %s", w.Code, w.Body.String())
	}

	// User dilarang ubah/hapus: 403.
	for _, m := range []string{http.MethodPut, http.MethodPatch, http.MethodDelete} {
		var body any
		if m != http.MethodDelete {
			body = payload
		}
		if w := doAuthRequest(t, e, m, path, user, body); w.Code != http.StatusForbidden {
			t.Fatalf("user %s status %d, want 403", m, w.Code)
		}
	}

	// Admin tetap boleh ubah/hapus.
	if w := doAuthRequest(t, e, http.MethodPut, path, admin, payload); w.Code != http.StatusOK {
		t.Fatalf("admin PUT status %d body %s", w.Code, w.Body.String())
	}
	if w := doAuthRequest(t, e, http.MethodDelete, path, admin, nil); w.Code != http.StatusNoContent {
		t.Fatalf("admin DELETE status %d, want 204", w.Code)
	}
}
