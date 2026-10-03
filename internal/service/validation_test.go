package service

import (
	"strings"
	"testing"
)

func validInput() BookingInput {
	return BookingInput{
		Tanggal:           "2026-10-10",
		Jam:               "19:00",
		Durasi:            120,
		NamaPenyewa:       "Budi Santoso",
		NominalPembayaran: 300000,
	}
}

func TestValidateFull_OK(t *testing.T) {
	parsed, verr := ValidateFull(validInput())
	if verr != nil {
		t.Fatalf("unexpected validation error: %+v", verr.Fields)
	}
	if parsed.NamaPenyewa != "Budi Santoso" || parsed.Durasi != 120 || parsed.Jam != "19:00" {
		t.Fatalf("parsed mismatch: %+v", parsed)
	}
	if got := parsed.Tanggal.Format("2006-01-02"); got != "2026-10-10" {
		t.Fatalf("tanggal = %q", got)
	}
}

func TestValidateFull_TrimNama(t *testing.T) {
	in := validInput()
	in.NamaPenyewa = "  Budi  "
	parsed, verr := ValidateFull(in)
	if verr != nil {
		t.Fatalf("unexpected error: %+v", verr.Fields)
	}
	if parsed.NamaPenyewa != "Budi" {
		t.Fatalf("nama tidak di-trim: %q", parsed.NamaPenyewa)
	}
}

func TestValidateFull_Errors(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*BookingInput)
		field  string
	}{
		{"tanggal kosong", func(i *BookingInput) { i.Tanggal = "" }, "tanggal"},
		{"tanggal format salah", func(i *BookingInput) { i.Tanggal = "10-10-2026" }, "tanggal"},
		{"jam kosong", func(i *BookingInput) { i.Jam = "" }, "jam"},
		{"jam lewat 24", func(i *BookingInput) { i.Jam = "25:00" }, "jam"},
		{"jam tanpa kolon", func(i *BookingInput) { i.Jam = "1900" }, "jam"},
		{"durasi nol", func(i *BookingInput) { i.Durasi = 0 }, "durasi"},
		{"durasi negatif", func(i *BookingInput) { i.Durasi = -30 }, "durasi"},
		{"durasi > 1440", func(i *BookingInput) { i.Durasi = 1441 }, "durasi"},
		{"nama kosong", func(i *BookingInput) { i.NamaPenyewa = "" }, "nama_penyewa"},
		{"nama spasi saja", func(i *BookingInput) { i.NamaPenyewa = "   " }, "nama_penyewa"},
		{"nama > 120", func(i *BookingInput) { i.NamaPenyewa = strings.Repeat("a", 121) }, "nama_penyewa"},
		{"nominal negatif", func(i *BookingInput) { i.NominalPembayaran = -1 }, "nominal_pembayaran"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := validInput()
			c.mutate(&in)
			_, verr := ValidateFull(in)
			if verr == nil {
				t.Fatal("expected validation error, got nil")
			}
			found := false
			for _, f := range verr.Fields {
				if f.Field == c.field {
					found = true
				}
			}
			if !found {
				t.Fatalf("expected error on field %q, got %+v", c.field, verr.Fields)
			}
		})
	}
}

func TestValidateFull_NominalNolOK(t *testing.T) {
	in := validInput()
	in.NominalPembayaran = 0
	if _, verr := ValidateFull(in); verr != nil {
		t.Fatalf("nominal 0 harus valid: %+v", verr.Fields)
	}
}

func TestParseDateParam(t *testing.T) {
	if _, ferr := ParseDateParam("cutoff_date", "", true); ferr == nil {
		t.Error("required kosong harus error")
	}
	if v, ferr := ParseDateParam("start_date", "", false); ferr != nil || v != nil {
		t.Error("optional kosong harus (nil, nil)")
	}
	if _, ferr := ParseDateParam("cutoff_date", "2026-13-40", true); ferr == nil {
		t.Error("format salah harus error")
	}
	if v, ferr := ParseDateParam("cutoff_date", "2026-10-31", true); ferr != nil || v == nil {
		t.Error("tanggal valid harus lolos")
	}
}
