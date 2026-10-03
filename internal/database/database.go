package database

import (
	"booking-manager/internal/config"
	"booking-manager/internal/model"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Connect(cfg *config.Config) (*gorm.DB, error) {
	var dialector gorm.Dialector
	if cfg.IsPostgres() {
		dialector = postgres.Open(cfg.DatabaseURL)
	} else {
		dsn := cfg.DatabaseURL
		if dsn == "" {
			dsn = "bookings.db"
		}
		dialector = sqlite.Open(dsn)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}
	return db, nil
}

func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&model.Booking{})
}
