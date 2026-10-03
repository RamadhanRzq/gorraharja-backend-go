package format

import (
	"testing"
	"time"
)

func TestRupiah(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "Rp 0"},
		{500, "Rp 500"},
		{1000, "Rp 1.000"},
		{1250000, "Rp 1.250.000"},
		{12600000, "Rp 12.600.000"},
		{300000, "Rp 300.000"},
	}
	for _, c := range cases {
		if got := Rupiah(c.in); got != c.want {
			t.Errorf("Rupiah(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTanggalID(t *testing.T) {
	d := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	if got := TanggalID(d); got != "10 Okt 2026" {
		t.Errorf("TanggalID = %q, want %q", got, "10 Okt 2026")
	}
}

func TestJamSelesai(t *testing.T) {
	cases := []struct {
		jam    string
		durasi int
		want   string
	}{
		{"19:00", 120, "21:00"},
		{"23:30", 60, "00:30"}, // lewat tengah malam: bungkus, tetap tanggal mulai
		{"10:00", 90, "11:30"},
	}
	for _, c := range cases {
		if got := JamSelesai(c.jam, c.durasi); got != c.want {
			t.Errorf("JamSelesai(%q,%d) = %q, want %q", c.jam, c.durasi, got, c.want)
		}
	}
}
