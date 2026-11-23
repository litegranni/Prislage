package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr           string
	DatabaseURL        string
	APIToken           string
	UserAgent          string
	FetchInterval      time.Duration
	FetchRatePerMinute int
	VAPIDPublicKey     string
	VAPIDPrivateKey    string
	VAPIDSubject       string
	CORSOrigin         string
	MigrationsDir      string
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:           env("HTTP_ADDR", ":3847"),
		DatabaseURL:        env("DATABASE_URL", "postgres://prislage:prislage@localhost:5432/prislage?sslmode=disable"),
		APIToken:           env("API_TOKEN", "dev-token-change-me"),
		UserAgent:          env("USER_AGENT", "PrislageBot/0.1 (+https://github.com/prislage/prislage; price-watch; contact=prislage@localhost)"),
		FetchInterval:      durationEnv("FETCH_INTERVAL", 24*time.Hour),
		FetchRatePerMinute: intEnv("FETCH_RATE_PER_MINUTE", 10),
		VAPIDPublicKey:     os.Getenv("VAPID_PUBLIC_KEY"),
		VAPIDPrivateKey:    os.Getenv("VAPID_PRIVATE_KEY"),
		VAPIDSubject:       env("VAPID_SUBJECT", "mailto:prislage@localhost"),
		CORSOrigin:         env("CORS_ORIGIN", "*"),
		MigrationsDir:      env("MIGRATIONS_DIR", "migrations"),
	}
	if cfg.DatabaseURL == "" {
		return cfg, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.APIToken == "" {
		return cfg, fmt.Errorf("API_TOKEN is required")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func intEnv(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
