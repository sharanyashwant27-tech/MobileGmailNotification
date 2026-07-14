package config_test

import (
	"testing"

	"github.com/yashs/mobile-gmail-notification/internal/config"
)

func TestLoadSuccess(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/db?sslmode=disable")
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	t.Setenv("TOKEN_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	t.Setenv("APP_PORT", "9090")
	t.Setenv("JWT_ACCESS_TTL", "15m")
	t.Setenv("JWT_REFRESH_TTL", "168h")

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AppPort != "9090" {
		t.Fatalf("port=%s", cfg.AppPort)
	}
	if len(cfg.TokenEncryptionKey) != 32 {
		t.Fatalf("key len=%d", len(cfg.TokenEncryptionKey))
	}
}

func TestValidateRejectsShortJWT(t *testing.T) {
	cfg := &config.Config{
		DatabaseURL:        "postgres://localhost/db",
		JWTSecret:          "too-short",
		TokenEncryptionKey: make([]byte, 32),
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for short JWT secret")
	}
}
