// Package config loads runtime configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultDevJWTSecret = "dev-insecure-jwt-secret-change-me-please-32"

// Config holds every runtime setting of the API server.
type Config struct {
	AppEnv   string
	Port     string
	LogLevel string
	Timezone string

	DatabaseURL string
	DBMaxConns  int32
	DBMinConns  int32

	JWTSecret     string
	JWTIssuer     string
	JWTAccessTTL  time.Duration
	JWTRefreshTTL time.Duration

	BookingPaymentDeadline    time.Duration
	BookingCancellationCutoff time.Duration
	BookingMaxAdvanceDays     int
	ExpirySweepInterval       time.Duration

	MigrateOnStart bool
	SeedOnStart    bool
	AdminEmail     string
	AdminPassword  string
	AdminName      string

	CORSOrigins     []string
	ShutdownTimeout time.Duration

	loc *time.Location
}

// Location returns the configured timezone used for booking/availability math.
func (c *Config) Location() *time.Location { return c.loc }

// IsProduction reports whether the process runs with production hardening.
func (c *Config) IsProduction() bool { return strings.EqualFold(c.AppEnv, "production") }

// Load reads the environment and validates the resulting configuration.
func Load() (*Config, error) {
	cfg := &Config{
		AppEnv:   env("APP_ENV", "development"),
		Port:     env("PORT", "8080"),
		LogLevel: env("LOG_LEVEL", "info"),
		Timezone: env("APP_TIMEZONE", "Asia/Jakarta"),

		DatabaseURL: env("DATABASE_URL", "postgres://spc:postgres@localhost:5432/gorraharja?sslmode=disable"),
		DBMaxConns:  int32(envInt("DB_MAX_CONNS", 10)),
		DBMinConns:  int32(envInt("DB_MIN_CONNS", 1)),

		JWTSecret:     env("JWT_SECRET", defaultDevJWTSecret),
		JWTIssuer:     env("JWT_ISSUER", "gorraharja-api"),
		JWTAccessTTL:  envDuration("JWT_ACCESS_TTL", 15*time.Minute),
		JWTRefreshTTL: envDuration("JWT_REFRESH_TTL", 7*24*time.Hour),

		BookingPaymentDeadline:    envDuration("BOOKING_PAYMENT_DEADLINE", 30*time.Minute),
		BookingCancellationCutoff: envDuration("BOOKING_CANCELLATION_CUTOFF", 0),
		BookingMaxAdvanceDays:     envInt("BOOKING_MAX_ADVANCE_DAYS", 30),
		ExpirySweepInterval:       envDuration("EXPIRY_SWEEP_INTERVAL", time.Minute),

		MigrateOnStart: envBool("MIGRATE_ON_START", true),
		SeedOnStart:    envBool("SEED_ON_START", true),
		AdminEmail:     env("ADMIN_EMAIL", "admin@gorraharja.local"),
		AdminPassword:  env("ADMIN_PASSWORD", "Admin#12345"),
		AdminName:      env("ADMIN_NAME", "GOR Administrator"),

		CORSOrigins:     envList("CORS_ORIGINS", []string{"http://localhost:5173"}),
		ShutdownTimeout: envDuration("SHUTDOWN_TIMEOUT", 15*time.Second),
	}

	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return nil, fmt.Errorf("invalid APP_TIMEZONE %q: %w", cfg.Timezone, err)
	}
	cfg.loc = loc

	if cfg.DBMaxConns < 1 {
		cfg.DBMaxConns = 1
	}
	if cfg.DBMinConns < 0 {
		cfg.DBMinConns = 0
	}
	if cfg.DBMinConns > cfg.DBMaxConns {
		cfg.DBMinConns = cfg.DBMaxConns
	}
	if cfg.BookingMaxAdvanceDays < 1 {
		cfg.BookingMaxAdvanceDays = 1
	}
	if cfg.ExpirySweepInterval <= 0 {
		cfg.ExpirySweepInterval = time.Minute
	}

	if cfg.IsProduction() {
		if cfg.JWTSecret == defaultDevJWTSecret || len(cfg.JWTSecret) < 32 {
			return nil, fmt.Errorf("JWT_SECRET must be a random value of at least 32 characters in production")
		}
		if cfg.SeedOnStart && cfg.AdminPassword == "Admin#12345" {
			return nil, fmt.Errorf("ADMIN_PASSWORD must be changed when SEED_ON_START is enabled in production")
		}
	}
	return cfg, nil
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return def
}

func envInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		if b, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
			return b
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		if d, err := time.ParseDuration(strings.TrimSpace(v)); err == nil {
			return d
		}
	}
	return def
}

func envList(key string, def []string) []string {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return def
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return def
	}
	return out
}
