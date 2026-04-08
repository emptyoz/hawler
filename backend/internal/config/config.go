package config

import (
	"os"
	"time"
)

type Config struct {
	HTTPAddr    string
	DatabaseURL string
	JWTSecret   string
	JWTTTL      time.Duration
}

func FromEnv() Config {
	jwtTTL := 72 * time.Hour
	if raw := os.Getenv("JWT_TTL"); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil {
			jwtTTL = parsed
		}
	}

	return Config{
		HTTPAddr:    envOrDefault("HTTP_ADDR", ":8080"),
		DatabaseURL: envOrDefault("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/hawler?sslmode=disable"),
		JWTSecret:   envOrDefault("JWT_SECRET", "dev-secret-change-me"),
		JWTTTL:      jwtTTL,
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
