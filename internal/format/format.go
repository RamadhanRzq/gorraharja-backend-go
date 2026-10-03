package format

import (
	"fmt"
	"strings"
	"time"
)

// Rupiah memformat integer rupiah menjadi "Rp 1.250.000".
func Rupiah(n int64) string {
	if n == 0 {
		return "Rp 0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	var b strings.Builder
	rem := len(s) % 3
	if rem == 0 {
		rem = 3
	}
	b.WriteString(s[:rem])
	for i := rem; i < len(s); i += 3 {
		b.WriteByte('.')
		b.WriteString(s[i : i+3])
	}
	if neg {
		return "Rp -" + b.String()
	}
	return "Rp " + b.String()
}

var bulanID = [12]string{
	"Jan", "Feb", "Mar", "Apr", "Mei", "Jun",
	"Jul", "Agu", "Sep", "Okt", "Nov", "Des",
}

// TanggalID memformat time menjadi "10 Okt 2026".
func TanggalID(t time.Time) string {
	return fmt.Sprintf("%d %s %d", t.Day(), bulanID[int(t.Month())-1], t.Year())
}

// JamSelesai menghitung jam selesai dari jam mulai "HH:MM" + durasi menit.
// Melewati tengah malam dibungkus mod 24 jam (tetap milik tanggal mulai).
func JamSelesai(jam string, durasi int) string {
	var h, m int
	_, _ = fmt.Sscanf(jam, "%d:%d", &h, &m)
	total := h*60 + m + durasi
	total %= 24 * 60
	return fmt.Sprintf("%02d:%02d", total/60, total%60)
}

// RentangJam memformat "19:00-21:00".
func RentangJam(jam string, durasi int) string {
	return jam + "-" + JamSelesai(jam, durasi)
}

// DurasiLabel memformat menit menjadi label ringkas.
func DurasiLabel(menit int) string {
	if menit%60 == 0 {
		return fmt.Sprintf("%d jam", menit/60)
	}
	return fmt.Sprintf("%d mnt", menit)
}
