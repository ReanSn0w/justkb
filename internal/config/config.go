package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddress   string
	DatabaseURL     string
	StoragePath     string
	PublicURL       string
	CookieSecure    bool
	ShutdownTimeout time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		ListenAddress:   envOrDefault("JKB_LISTEN_ADDRESS", ":8080"),
		DatabaseURL:     strings.TrimSpace(os.Getenv("JKB_DATABASE_URL")),
		StoragePath:     envOrDefault("JKB_STORAGE_PATH", "./storage"),
		PublicURL:       strings.TrimRight(strings.TrimSpace(os.Getenv("JKB_PUBLIC_URL")), "/"),
		CookieSecure:    true,
		ShutdownTimeout: 15 * time.Second,
	}

	if raw := strings.TrimSpace(os.Getenv("JKB_COOKIE_SECURE")); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("JKB_COOKIE_SECURE: %w", err)
		}
		cfg.CookieSecure = value
	}
	if raw := strings.TrimSpace(os.Getenv("JKB_SHUTDOWN_TIMEOUT")); raw != "" {
		value, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("JKB_SHUTDOWN_TIMEOUT: %w", err)
		}
		cfg.ShutdownTimeout = value
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("JKB_DATABASE_URL is required")
	}
	if cfg.StoragePath == "" {
		return Config{}, errors.New("JKB_STORAGE_PATH is required")
	}
	if cfg.ShutdownTimeout <= 0 {
		return Config{}, errors.New("JKB_SHUTDOWN_TIMEOUT must be positive")
	}
	return cfg, nil
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
