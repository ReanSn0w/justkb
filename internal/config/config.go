package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddress     string
	DatabasePath      string
	StoragePath       string
	SystemName        string
	IconColor         string
	PublicURL         string
	CookieSecure      bool
	AttachmentMaxSize int64
	ShutdownTimeout   time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		ListenAddress:     envOrDefault("JKB_LISTEN_ADDRESS", ":8080"),
		DatabasePath:      envOrDefault("JKB_DATABASE_PATH", "./storage/jkb.sqlite"),
		StoragePath:       envOrDefault("JKB_STORAGE_PATH", "./storage/files"),
		SystemName:        envOrDefault("JKB_SYSTEM_NAME", "JKB"),
		IconColor:         strings.ToUpper(envOrDefault("JKB_ICON_COLOR", "#2563EB")),
		PublicURL:         strings.TrimRight(strings.TrimSpace(os.Getenv("JKB_PUBLIC_URL")), "/"),
		CookieSecure:      true,
		AttachmentMaxSize: 25 << 20,
		ShutdownTimeout:   15 * time.Second,
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
	if raw := strings.TrimSpace(os.Getenv("JKB_ATTACHMENT_MAX_BYTES")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value <= 0 {
			return Config{}, errors.New("JKB_ATTACHMENT_MAX_BYTES must be a positive integer")
		}
		cfg.AttachmentMaxSize = value
	}

	if !regexp.MustCompile(`^#[0-9A-F]{6}$`).MatchString(cfg.IconColor) {
		return Config{}, errors.New("JKB_ICON_COLOR must be #RRGGBB")
	}
	if cfg.DatabasePath == "" {
		return Config{}, errors.New("JKB_DATABASE_PATH is required")
	}
	if cfg.StoragePath == "" {
		return Config{}, errors.New("JKB_STORAGE_PATH is required")
	}
	if len([]rune(cfg.SystemName)) > 80 {
		return Config{}, errors.New("JKB_SYSTEM_NAME must not exceed 80 characters")
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
