package handler

import (
	"net/http"
	"strconv"
	"time"

	"booking-manager/internal/middleware"
	"booking-manager/internal/model"
	"booking-manager/internal/pdf"
	"booking-manager/internal/repository"
	"booking-manager/internal/service"

	"github.com/gin-gonic/gin"
)

type BookingHandler struct {
	svc service.BookingService
}

func NewBookingHandler(svc service.BookingService) *BookingHandler {
	return &BookingHandler{svc: svc}
}

// BookingResponse adalah representasi JSON sesuai PRD §8.
type BookingResponse struct {
	ID                uint   `json:"id"`
	Tanggal           string `json:"tanggal"`
	Jam               string `json:"jam"`
	Durasi            int    `json:"durasi"`
	NamaPenyewa       string `json:"nama_penyewa"`
	NominalPembayaran int64  `json:"nominal_pembayaran"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

func toResponse(b *model.Booking) BookingResponse {
	return BookingResponse{
		ID:                b.ID,
		Tanggal:           b.Tanggal.Format("2006-01-02"),
		Jam:               b.Jam,
		Durasi:            b.Durasi,
		NamaPenyewa:       b.NamaPenyewa,
		NominalPembayaran: b.NominalPembayaran,
		CreatedAt:         b.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:         b.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func errorJSON(c *gin.Context, status int, code, message string, details []service.FieldError) {
	body := gin.H{"code": code, "message": message}
	if details != nil {
		rows := make([]gin.H, 0, len(details))
		for _, d := range details {
			rows = append(rows, gin.H{"field": d.Field, "message": d.Message})
		}
		body["details"] = rows
	}
	c.JSON(status, gin.H{"error": body})
}

func writeServiceError(c *gin.Context, err error) {
	switch e := err.(type) {
	case *service.ValidationError:
		errorJSON(c, http.StatusBadRequest, "VALIDATION_ERROR", "Data tidak valid", e.Fields)
	case *service.NotFoundError:
		errorJSON(c, http.StatusNotFound, "NOT_FOUND", e.Error(), nil)
	default:
		errorJSON(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Terjadi kesalahan server", nil)
	}
}

// Me → GET /api/v1/me (peran hasil autentikasi).
func (h *BookingHandler) Me(c *gin.Context) {
	role := middleware.GetRole(c)
	if role == "" {
		role = middleware.RoleAdmin
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"role": role}})
}

func parseID(c *gin.Context) (uint, bool) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id64 == 0 {
		errorJSON(c, http.StatusBadRequest, "VALIDATION_ERROR", "ID tidak valid",
			[]service.FieldError{{Field: "id", Message: "harus angka positif"}})
		return 0, false
	}
	return uint(id64), true
}

// numberValue mengambil angka dari beberapa kemungkinan nama kunci.
// Frontend boleh mengirim alias: durasi|durasi_menit, nominal|nominal_pembayaran.
func numberValue(raw map[string]any, keys ...string) (float64, bool) {
	for _, k := range keys {
		v, ok := raw[k]
		if !ok {
			continue
		}
		switch n := v.(type) {
		case float64:
			return n, true
		case float32:
			return float64(n), true
		case int:
			return float64(n), true
		case int64:
			return float64(n), true
		}
	}
	return 0, false
}

func stringValue(raw map[string]any, keys ...string) (string, bool) {
	for _, k := range keys {
		if v, ok := raw[k].(string); ok {
			return v, true
		}
	}
	return "", false
}

// decodeFull memetakan payload create/PUT penuh, toleran terhadap alias
// kunci yang dikirim frontend (durasi_menit, nominal).
func decodeFull(raw map[string]any) service.BookingInput {
	var in service.BookingInput
	if v, ok := stringValue(raw, "tanggal"); ok {
		in.Tanggal = v
	}
	if v, ok := stringValue(raw, "jam"); ok {
		in.Jam = v
	}
	if v, ok := numberValue(raw, "durasi", "durasi_menit"); ok {
		in.Durasi = int(v)
	}
	if v, ok := stringValue(raw, "nama_penyewa", "nama"); ok {
		in.NamaPenyewa = v
	}
	if v, ok := numberValue(raw, "nominal_pembayaran", "nominal"); ok {
		in.NominalPembayaran = int64(v)
	}
	return in
}

// decodePatch memetakan payload PATCH sebagian dengan alias yang sama.
func decodePatch(raw map[string]any) service.BookingPatch {
	var p service.BookingPatch
	if v, ok := stringValue(raw, "tanggal"); ok {
		p.Tanggal = &v
	}
	if v, ok := stringValue(raw, "jam"); ok {
		p.Jam = &v
	}
	if v, ok := numberValue(raw, "durasi", "durasi_menit"); ok {
		d := int(v)
		p.Durasi = &d
	}
	if v, ok := stringValue(raw, "nama_penyewa", "nama"); ok {
		p.NamaPenyewa = &v
	}
	if v, ok := numberValue(raw, "nominal_pembayaran", "nominal"); ok {
		n := int64(v)
		p.NominalPembayaran = &n
	}
	return p
}

// Create → POST /bookings (201).
func (h *BookingHandler) Create(c *gin.Context) {
	// Binding manual agar error format tetap konsisten (bukan pesan bawaan Gin).
	var raw map[string]any
	if err := c.ShouldBindJSON(&raw); err != nil {
		errorJSON(c, http.StatusBadRequest, "VALIDATION_ERROR", "Body JSON tidak valid", nil)
		return
	}
	in := decodeFull(raw)
	b, err := h.svc.Create(in)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": toResponse(b)})
}

// List → GET /bookings?start_date=&end_date=&q=&page=&limit=.
func (h *BookingHandler) List(c *gin.Context) {
	var details []service.FieldError

	startDate, ferr := service.ParseDateParam("start_date", c.Query("start_date"), false)
	if ferr != nil {
		details = append(details, *ferr)
	}
	endDate, ferr := service.ParseDateParam("end_date", c.Query("end_date"), false)
	if ferr != nil {
		details = append(details, *ferr)
	}
	if startDate != nil && endDate != nil && startDate.After(*endDate) {
		details = append(details, service.FieldError{Field: "end_date", Message: "harus >= start_date"})
	}

	page := 1
	if s := c.Query("page"); s != "" {
		p, err := strconv.Atoi(s)
		if err != nil || p < 1 {
			details = append(details, service.FieldError{Field: "page", Message: "harus >= 1"})
		} else {
			page = p
		}
	}
	limit := 20
	if s := c.Query("limit"); s != "" {
		l, err := strconv.Atoi(s)
		if err != nil || l < 1 || l > 100 {
			details = append(details, service.FieldError{Field: "limit", Message: "harus antara 1 dan 100"})
		} else {
			limit = l
		}
	}
	if len(details) > 0 {
		errorJSON(c, http.StatusBadRequest, "VALIDATION_ERROR", "Parameter tidak valid", details)
		return
	}

	items, total, totalNominal, err := h.svc.List(repository.ListFilter{
		StartDate: startDate,
		EndDate:   endDate,
		Q:         c.Query("q"),
		Page:      page,
		Limit:     limit,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	data := make([]BookingResponse, 0, len(items))
	for i := range items {
		data = append(data, toResponse(&items[i]))
	}
	c.JSON(http.StatusOK, gin.H{
		"data": data,
		"meta": gin.H{
			"page":          page,
			"limit":         limit,
			"total":         total,
			"total_nominal": totalNominal,
		},
	})
}

// Detail → GET /bookings/:id.
func (h *BookingHandler) Detail(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	b, err := h.svc.Get(id)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": toResponse(b)})
}

// Put → PUT /bookings/:id (penuh).
func (h *BookingHandler) Put(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var raw map[string]any
	if err := c.ShouldBindJSON(&raw); err != nil {
		errorJSON(c, http.StatusBadRequest, "VALIDATION_ERROR", "Body JSON tidak valid", nil)
		return
	}
	in := decodeFull(raw)
	b, err := h.svc.UpdateFull(id, in)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": toResponse(b)})
}

// Patch → PATCH /bookings/:id (sebagian).
func (h *BookingHandler) Patch(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var raw map[string]any
	if err := c.ShouldBindJSON(&raw); err != nil {
		errorJSON(c, http.StatusBadRequest, "VALIDATION_ERROR", "Body JSON tidak valid", nil)
		return
	}
	p := decodePatch(raw)
	b, err := h.svc.UpdatePatch(id, p)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": toResponse(b)})
}

// Delete → DELETE /bookings/:id (204, soft delete).
func (h *BookingHandler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(id); err != nil {
		writeServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ExportPDF → GET /bookings/export/pdf?cutoff_date=&start_date=.
func (h *BookingHandler) ExportPDF(c *gin.Context) {
	var details []service.FieldError
	cutoff, ferr := service.ParseDateParam("cutoff_date", c.Query("cutoff_date"), true)
	if ferr != nil {
		details = append(details, *ferr)
	}
	var start time.Time
	hasStart := c.Query("start_date") != ""
	if hasStart {
		s, ferr := service.ParseDateParam("start_date", c.Query("start_date"), true)
		if ferr != nil {
			details = append(details, *ferr)
		} else {
			start = *s
		}
	}
	if len(details) > 0 {
		errorJSON(c, http.StatusBadRequest, "VALIDATION_ERROR", "Parameter tidak valid", details)
		return
	}
	if hasStart && start.After(*cutoff) {
		errorJSON(c, http.StatusBadRequest, "VALIDATION_ERROR", "Parameter tidak valid",
			[]service.FieldError{{Field: "start_date", Message: "harus <= cutoff_date"}})
		return
	}

	items, effectiveStart, err := h.svc.ExportRange(start, *cutoff, hasStart)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	out, err := pdf.Rekap(items, effectiveStart, *cutoff, time.Now())
	if err != nil {
		writeServiceError(c, err)
		return
	}
	filename := "rekap-booking-" + effectiveStart.Format("2006-01-02") + "_" + cutoff.Format("2006-01-02") + ".pdf"
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	c.Data(http.StatusOK, "application/pdf", out)
}
