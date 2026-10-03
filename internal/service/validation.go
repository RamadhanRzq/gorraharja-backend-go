package service

import (
	"fmt"
	"strings"
	"time"
)

// FieldError adalah satu kesalahan validasi per field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationError dikembalikan service saat input tidak valid.
type ValidationError struct {
	Fields []FieldError
}

func (e *ValidationError) Error() string {
	if len(e.Fields) == 0 {
		return "data tidak valid"
	}
	return fmt.Sprintf("data tidak valid: %s", e.Fields[0].Field)
}

// NotFoundError dikembalikan saat booking tidak ada.
type NotFoundError struct {
	ID uint
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("booking dengan id %d tidak ditemukan", e.ID)
}

// BookingInput adalah payload create / PUT penuh.
type BookingInput struct {
	Tanggal           string `json:"tanggal"`
	Jam               string `json:"jam"`
	Durasi            int    `json:"durasi"`
	NamaPenyewa       string `json:"nama_penyewa"`
	NominalPembayaran int64  `json:"nominal_pembayaran"`
}

// BookingPatch adalah payload PATCH sebagian; nil = tidak diubah.
type BookingPatch struct {
	Tanggal           *string `json:"tanggal"`
	Jam               *string `json:"jam"`
	Durasi            *int    `json:"durasi"`
	NamaPenyewa       *string `json:"nama_penyewa"`
	NominalPembayaran *int64  `json:"nominal_pembayaran"`
}

// ParsedBooking adalah hasil validasi + parsing yang siap disimpan.
type ParsedBooking struct {
	Tanggal           time.Time
	Jam               string
	Durasi            int
	NamaPenyewa       string
	NominalPembayaran int64
}

// ValidateFull memvalidasi payload penuh (create & PUT).
func ValidateFull(in BookingInput) (ParsedBooking, *ValidationError) {
	var out ParsedBooking
	var verr ValidationError

	t, ok := parseTanggal(in.Tanggal)
	if !ok {
		verr.Fields = append(verr.Fields, FieldError{Field: "tanggal", Message: "format harus YYYY-MM-DD"})
	} else {
		out.Tanggal = t
	}
	if !validJam(in.Jam) {
		verr.Fields = append(verr.Fields, FieldError{Field: "jam", Message: "format harus HH:MM (00:00-23:59)"})
	} else {
		out.Jam = in.Jam
	}
	if in.Durasi <= 0 || in.Durasi > 1440 {
		verr.Fields = append(verr.Fields, FieldError{Field: "durasi", Message: "harus antara 1 dan 1440 menit"})
	} else {
		out.Durasi = in.Durasi
	}
	nama := strings.TrimSpace(in.NamaPenyewa)
	if nama == "" {
		verr.Fields = append(verr.Fields, FieldError{Field: "nama_penyewa", Message: "wajib diisi"})
	} else if len([]rune(nama)) > 120 {
		verr.Fields = append(verr.Fields, FieldError{Field: "nama_penyewa", Message: "maksimal 120 karakter"})
	} else {
		out.NamaPenyewa = nama
	}
	if in.NominalPembayaran < 0 {
		verr.Fields = append(verr.Fields, FieldError{Field: "nominal_pembayaran", Message: "minimal 0"})
	} else {
		out.NominalPembayaran = in.NominalPembayaran
	}

	if len(verr.Fields) > 0 {
		return ParsedBooking{}, &verr
	}
	return out, nil
}

func parseTanggal(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func validJam(s string) bool {
	if len(s) != 5 || s[2] != ':' {
		return false
	}
	h := (s[0]-'0')*10 + (s[1] - '0')
	m := (s[3]-'0')*10 + (s[4] - '0')
	if s[0] < '0' || s[0] > '9' || s[1] < '0' || s[1] > '9' ||
		s[3] < '0' || s[3] > '9' || s[4] < '0' || s[4] > '9' {
		return false
	}
	return h <= 23 && m <= 59
}

// ParseDateParam memvalidasi parameter query tanggal (boleh kosong bila optional).
func ParseDateParam(field, value string, required bool) (*time.Time, *FieldError) {
	if value == "" {
		if required {
			return nil, &FieldError{Field: field, Message: "wajib diisi (format YYYY-MM-DD)"}
		}
		return nil, nil
	}
	t, ok := parseTanggal(value)
	if !ok {
		return nil, &FieldError{Field: field, Message: "format harus YYYY-MM-DD"}
	}
	return &t, nil
}
