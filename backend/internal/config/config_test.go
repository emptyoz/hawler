package config

import (
	"testing"
	"time"
)

func TestEnvOrDefault(t *testing.T) {
	t.Setenv("TEST_KEY", "")
	if got := envOrDefault("TEST_KEY", "fallback"); got != "fallback" {
		t.Fatalf("envOrDefault() = %q, want %q", got, "fallback")
	}

	t.Setenv("TEST_KEY", "value")
	if got := envOrDefault("TEST_KEY", "fallback"); got != "value" {
		t.Fatalf("envOrDefault() = %q, want %q", got, "value")
	}
}

func TestFromEnvDefaults(t *testing.T) {
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("JWT_TTL", "")

	cfg := FromEnv()
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want %q", cfg.HTTPAddr, ":8080")
	}
	if cfg.DatabaseURL != "postgres://postgres:postgres@localhost:5432/hawler?sslmode=disable" {
		t.Fatalf("DatabaseURL = %q, want default", cfg.DatabaseURL)
	}
	if cfg.JWTSecret != "dev-secret-change-me" {
		t.Fatalf("JWTSecret = %q, want %q", cfg.JWTSecret, "dev-secret-change-me")
	}
	if cfg.JWTTTL != 72*time.Hour {
		t.Fatalf("JWTTTL = %s, want %s", cfg.JWTTTL, 72*time.Hour)
	}
}

func TestFromEnvOverridesAndTTLParse(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":9000")
	t.Setenv("DATABASE_URL", "postgres://custom")
	t.Setenv("JWT_SECRET", "custom-secret")
	t.Setenv("JWT_TTL", "24h")

	cfg := FromEnv()
	if cfg.HTTPAddr != ":9000" {
		t.Fatalf("HTTPAddr = %q, want %q", cfg.HTTPAddr, ":9000")
	}
	if cfg.DatabaseURL != "postgres://custom" {
		t.Fatalf("DatabaseURL = %q, want %q", cfg.DatabaseURL, "postgres://custom")
	}
	if cfg.JWTSecret != "custom-secret" {
		t.Fatalf("JWTSecret = %q, want %q", cfg.JWTSecret, "custom-secret")
	}
	if cfg.JWTTTL != 24*time.Hour {
		t.Fatalf("JWTTTL = %s, want %s", cfg.JWTTTL, 24*time.Hour)
	}
}

func TestFromEnvInvalidTTLUsesDefault(t *testing.T) {
	t.Setenv("JWT_TTL", "not-a-duration")

	cfg := FromEnv()
	if cfg.JWTTTL != 72*time.Hour {
		t.Fatalf("JWTTTL = %s, want %s", cfg.JWTTTL, 72*time.Hour)
	}
}
