package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	DatabaseURL string
	// APIKey legacy: bila ADMIN_API_KEY kosong, dipakai sebagai kunci admin.
	APIKey      string
	AdminAPIKey string
	UserAPIKey  string
	Env         string
}

func Load() *Config {
	_ = godotenv.Load()

	return &Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		APIKey:      os.Getenv("API_KEY"),
		AdminAPIKey: firstNonEmpty(os.Getenv("ADMIN_API_KEY"), os.Getenv("API_KEY")),
		UserAPIKey:  os.Getenv("USER_API_KEY"),
		Env:         getEnv("APP_ENV", "development"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

// IsPostgres reports whether DATABASE_URL points to PostgreSQL.
func (c *Config) IsPostgres() bool {
	u := c.DatabaseURL
	if len(u) >= 8 && u[:8] == "postgres" {
		return true
	}
	return false
}
