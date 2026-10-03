package pdf

import (
	"bytes"
	"testing"
	"time"

	"booking-manager/internal/model"
)

func booking(tanggal string, jam string, durasi int, nama string, nominal int64) model.Booking {
	t, _ := time.Parse("2006-01-02", tanggal)
	return model.Booking{
		Tanggal:           t,
		Jam:               jam,
		Durasi:            durasi,
		NamaPenyewa:       nama,
		NominalPembayaran: nominal,
	}
}

func TestRekap_DataAda(t *testing.T) {
	items := []model.Booking{
		booking("2026-10-10", "19:00", 120, "Budi Santoso", 300000),
		booking("2026-10-11", "08:00", 60, "Siti", 150000),
	}
	start, _ := time.Parse("2006-01-02", "2026-10-01")
	cutoff, _ := time.Parse("2006-01-02", "2026-10-31")

	out, err := Rekap(items, start, cutoff, time.Now())
	if err != nil {
		t.Fatalf("Rekap error: %v", err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF")) {
		t.Fatal("output bukan PDF")
	}
	if len(out) < 1000 {
		t.Fatalf("PDF terlalu kecil (%d byte), tabel mungkin tidak terisi", len(out))
	}
}

func TestRekap_Kosong(t *testing.T) {
	start, _ := time.Parse("2006-01-02", "2026-11-01")
	cutoff, _ := time.Parse("2006-01-02", "2026-11-30")

	out, err := Rekap(nil, start, cutoff, time.Now())
	if err != nil {
		t.Fatalf("Rekap error: %v", err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF")) {
		t.Fatal("PDF kosong tetap harus dihasilkan")
	}
}
