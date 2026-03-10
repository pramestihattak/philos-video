package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port        int
	DatabaseURL string
	DataDir     string
	WorkerCount int
	JWTSecret   string
	JWTExpiry   string
	RTMPPort    int
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:        8080,
		DatabaseURL: "postgres://philos:philos@localhost:5433/philos_video?sslmode=disable",
		DataDir:     "./data",
		WorkerCount: 2,
		JWTSecret:   "dev-secret-change-in-production-min-32-chars!",
		JWTExpiry:   "1h",
		RTMPPort:    1935,
	}

	if v := os.Getenv("PORT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid PORT: %w", err)
		}
		cfg.Port = n
	}
	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.DatabaseURL = v
	}
	if v := os.Getenv("DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("WORKER_COUNT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid WORKER_COUNT: %w", err)
		}
		cfg.WorkerCount = n
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.JWTSecret = v
	}
	if v := os.Getenv("JWT_EXPIRY"); v != "" {
		cfg.JWTExpiry = v
	}
	if v := os.Getenv("RTMP_PORT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid RTMP_PORT: %w", err)
		}
		cfg.RTMPPort = n
	}

	return cfg, nil
}
