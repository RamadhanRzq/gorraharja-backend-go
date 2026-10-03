package model

import (
	"time"

	"gorm.io/gorm"
)

// Booking adalah entitas sewa beserta nominal pembayarannya.
// Tanggal disimpan sebagai DATE (waktu lokal apa adanya),
// CreatedAt/UpdatedAt disimpan UTC oleh GORM.
type Booking struct {
	ID                uint           `gorm:"primaryKey" json:"id"`
	Tanggal           time.Time      `gorm:"type:date;index:idx_bookings_tanggal;index:idx_bookings_tanggal_jam,priority:1" json:"tanggal"`
	Jam               string         `gorm:"type:varchar(5);index:idx_bookings_tanggal_jam,priority:2" json:"jam"`
	Durasi            int            `gorm:"not null" json:"durasi"`
	NamaPenyewa       string         `gorm:"type:varchar(120);not null" json:"nama_penyewa"`
	NominalPembayaran int64          `gorm:"not null" json:"nominal_pembayaran"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName memastikan nama tabel tunggal sesuai PRD.
func (Booking) TableName() string { return "bookings" }
