package repository

import (
	"booking-manager/internal/model"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ListFilter adalah parameter filter/pagination untuk daftar booking.
type ListFilter struct {
	StartDate *time.Time
	EndDate   *time.Time
	Q         string
	Page      int
	Limit     int
}

// BookingRepository adalah kontrak akses data booking.
type BookingRepository interface {
	Create(b *model.Booking) error
	FindByID(id uint) (*model.Booking, error)
	List(f ListFilter) (items []model.Booking, total int64, totalNominal int64, err error)
	ListRange(start, end *time.Time) ([]model.Booking, error)
	EarliestDate() (*time.Time, error)
	Update(b *model.Booking) error
	Delete(id uint) error
}

type bookingRepository struct {
	db *gorm.DB
}

func NewBookingRepository(db *gorm.DB) BookingRepository {
	return &bookingRepository{db: db}
}

func (r *bookingRepository) scoped(f ListFilter) *gorm.DB {
	tx := r.db.Model(&model.Booking{})
	if f.StartDate != nil {
		tx = tx.Where("tanggal >= ?", *f.StartDate)
	}
	if f.EndDate != nil {
		tx = tx.Where("tanggal <= ?", *f.EndDate)
	}
	if f.Q != "" {
		// LOWER() agar case-insensitive di SQLite maupun PostgreSQL;
		// escape % _ \ agar q diperlakukan literal.
		q := strings.ToLower(escapeLike(f.Q))
		tx = tx.Where("LOWER(nama_penyewa) LIKE ? ESCAPE '\\'", "%"+q+"%")
	}
	return tx
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}

func (r *bookingRepository) Create(b *model.Booking) error {
	return r.db.Create(b).Error
}

func (r *bookingRepository) FindByID(id uint) (*model.Booking, error) {
	var b model.Booking
	if err := r.db.First(&b, id).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *bookingRepository) List(f ListFilter) ([]model.Booking, int64, int64, error) {
	// Query terpisah per agregat: statement GORM tidak boleh dipakai ulang
	// setelah Count/Select karena klausa menumpuk dan Scan int64
	// tertuju ke model Booking.
	var total int64
	if err := r.scoped(f).Count(&total).Error; err != nil {
		return nil, 0, 0, err
	}

	var totalNominal int64
	// Row().Scan melewati mapping model GORM; COALESCE menjamin 1 baris 0 saat kosong.
	if err := r.scoped(f).Select("COALESCE(SUM(nominal_pembayaran),0)").Row().Scan(&totalNominal); err != nil {
		return nil, 0, 0, err
	}

	var items []model.Booking
	offset := (f.Page - 1) * f.Limit
	if err := r.scoped(f).Order("tanggal ASC, jam ASC").Offset(offset).Limit(f.Limit).Find(&items).Error; err != nil {
		return nil, 0, 0, err
	}
	return items, total, totalNominal, nil
}

func (r *bookingRepository) ListRange(start, end *time.Time) ([]model.Booking, error) {
	tx := r.db.Model(&model.Booking{})
	if start != nil {
		tx = tx.Where("tanggal >= ?", *start)
	}
	if end != nil {
		tx = tx.Where("tanggal <= ?", *end)
	}
	var items []model.Booking
	if err := tx.Order("tanggal ASC, jam ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *bookingRepository) EarliestDate() (*time.Time, error) {
	var t *time.Time
	if err := r.db.Model(&model.Booking{}).Select("MIN(tanggal)").Scan(&t).Error; err != nil {
		return nil, err
	}
	return t, nil
}

func (r *bookingRepository) Update(b *model.Booking) error {
	return r.db.Save(b).Error
}

func (r *bookingRepository) Delete(id uint) error {
	return r.db.Delete(&model.Booking{}, id).Error
}
