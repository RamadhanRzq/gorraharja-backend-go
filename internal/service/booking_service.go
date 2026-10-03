package service

import (
	"errors"
	"time"

	"booking-manager/internal/model"
	"booking-manager/internal/repository"

	"gorm.io/gorm"
)

type BookingService interface {
	Create(in BookingInput) (*model.Booking, error)
	Get(id uint) (*model.Booking, error)
	List(f repository.ListFilter) ([]model.Booking, int64, int64, error)
	UpdateFull(id uint, in BookingInput) (*model.Booking, error)
	UpdatePatch(id uint, p BookingPatch) (*model.Booking, error)
	Delete(id uint) error
	// ExportRange mengambil seluruh booking inklusif [start, cutoff].
	// Bila start nil, dipakai tanggal booking paling awal (atau cutoff bila kosong).
	ExportRange(start, cutoff time.Time, hasStart bool) ([]model.Booking, time.Time, error)
}

type bookingService struct {
	repo repository.BookingRepository
}

func NewBookingService(repo repository.BookingRepository) BookingService {
	return &bookingService{repo: repo}
}

func (s *bookingService) Create(in BookingInput) (*model.Booking, error) {
	parsed, verr := ValidateFull(in)
	if verr != nil {
		return nil, verr
	}
	b := &model.Booking{
		Tanggal:           parsed.Tanggal,
		Jam:               parsed.Jam,
		Durasi:            parsed.Durasi,
		NamaPenyewa:       parsed.NamaPenyewa,
		NominalPembayaran: parsed.NominalPembayaran,
	}
	if err := s.repo.Create(b); err != nil {
		return nil, err
	}
	return b, nil
}

func (s *bookingService) Get(id uint) (*model.Booking, error) {
	b, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &NotFoundError{ID: id}
		}
		return nil, err
	}
	return b, nil
}

func (s *bookingService) List(f repository.ListFilter) ([]model.Booking, int64, int64, error) {
	return s.repo.List(f)
}

func (s *bookingService) UpdateFull(id uint, in BookingInput) (*model.Booking, error) {
	b, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	parsed, verr := ValidateFull(in)
	if verr != nil {
		return nil, verr
	}
	b.Tanggal = parsed.Tanggal
	b.Jam = parsed.Jam
	b.Durasi = parsed.Durasi
	b.NamaPenyewa = parsed.NamaPenyewa
	b.NominalPembayaran = parsed.NominalPembayaran
	if err := s.repo.Update(b); err != nil {
		return nil, err
	}
	return b, nil
}

func (s *bookingService) UpdatePatch(id uint, p BookingPatch) (*model.Booking, error) {
	b, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	// Bangun input penuh dari state saat ini + field yang diubah, lalu validasi sekali.
	current := BookingInput{
		Tanggal:           b.Tanggal.Format("2006-01-02"),
		Jam:               b.Jam,
		Durasi:            b.Durasi,
		NamaPenyewa:       b.NamaPenyewa,
		NominalPembayaran: b.NominalPembayaran,
	}
	if p.Tanggal != nil {
		current.Tanggal = *p.Tanggal
	}
	if p.Jam != nil {
		current.Jam = *p.Jam
	}
	if p.Durasi != nil {
		current.Durasi = *p.Durasi
	}
	if p.NamaPenyewa != nil {
		current.NamaPenyewa = *p.NamaPenyewa
	}
	if p.NominalPembayaran != nil {
		current.NominalPembayaran = *p.NominalPembayaran
	}
	parsed, verr := ValidateFull(current)
	if verr != nil {
		return nil, verr
	}
	b.Tanggal = parsed.Tanggal
	b.Jam = parsed.Jam
	b.Durasi = parsed.Durasi
	b.NamaPenyewa = parsed.NamaPenyewa
	b.NominalPembayaran = parsed.NominalPembayaran
	if err := s.repo.Update(b); err != nil {
		return nil, err
	}
	return b, nil
}

func (s *bookingService) Delete(id uint) error {
	if _, err := s.Get(id); err != nil {
		return err
	}
	return s.repo.Delete(id)
}

func (s *bookingService) ExportRange(start, cutoff time.Time, hasStart bool) ([]model.Booking, time.Time, error) {
	effectiveStart := start
	if !hasStart {
		earliest, err := s.repo.EarliestDate()
		if err != nil {
			return nil, time.Time{}, err
		}
		if earliest != nil {
			effectiveStart = *earliest
		} else {
			effectiveStart = cutoff
		}
	}
	items, err := s.repo.ListRange(&effectiveStart, &cutoff)
	if err != nil {
		return nil, time.Time{}, err
	}
	return items, effectiveStart, nil
}
