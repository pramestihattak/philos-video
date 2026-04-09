package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/caarlos0/env/v11"
)

const defaultJWTSecret = "dev-secret-change-in-production-min-32-chars!"

type Config struct {
	Port        int    `env:"PORT"         envDefault:"8080"`
	DatabaseURL string `env:"DATABASE_URL" envDefault:"postgres://philos:philos@localhost:5433/philos_video?sslmode=disable"`
	DataDir     string `env:"DATA_DIR"     envDefault:"./data"`
	WorkerCount int    `env:"WORKER_COUNT" envDefault:"2"`
	JWTSecret   string `env:"JWT_SECRET"   envDefault:"dev-secret-change-in-production-min-32-chars!"`
	JWTExpiry   string `env:"JWT_EXPIRY"   envDefault:"1h"`
	RTMPPort    int    `env:"RTMP_PORT"    envDefault:"1935"`
	LogLevel    string `env:"LOG_LEVEL"    envDefault:"info"`
	LogFormat   string `env:"LOG_FORMAT"   envDefault:"text"`

	// Google OAuth
	GoogleClientID     string `env:"GOOGLE_CLIENT_ID"`
	GoogleClientSecret string `env:"GOOGLE_CLIENT_SECRET"`
	OAuthRedirectURL   string `env:"OAUTH_REDIRECT_URL"`

	// Session cookie (signs the user-identity JWT stored in the browser cookie)
	SessionCookieSecret string `env:"SESSION_COOKIE_SECRET"`
	SessionCookieSecure bool   `env:"SESSION_COOKIE_SECURE" envDefault:"false"`

	// Per-user upload quota (bytes). 0 = unlimited.
	DefaultUploadQuotaBytes int64 `env:"DEFAULT_UPLOAD_QUOTA_BYTES" envDefault:"10737418240"`
}

func Load() (*Config, error) {
	// Load .env before parsing so real env vars still take precedence.
	if err := loadDotEnv(".env"); err != nil {
		return nil, fmt.Errorf("loading .env: %w", err)
	}

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	// Security: refuse to start with the well-known default JWT secret.
	if cfg.JWTSecret == defaultJWTSecret {
		return nil, fmt.Errorf("JWT_SECRET must be set; refusing to start with insecure default")
	}

	// Google OAuth vars are mandatory.
	if cfg.GoogleClientID == "" {
		return nil, fmt.Errorf("GOOGLE_CLIENT_ID must be set")
	}
	if cfg.GoogleClientSecret == "" {
		return nil, fmt.Errorf("GOOGLE_CLIENT_SECRET must be set")
	}
	if cfg.OAuthRedirectURL == "" {
		return nil, fmt.Errorf("OAUTH_REDIRECT_URL must be set")
	}
	if len(cfg.SessionCookieSecret) < 32 {
		return nil, fmt.Errorf("SESSION_COOKIE_SECRET must be at least 32 characters")
	}

	return cfg, nil
}

// loadDotEnv reads KEY=VALUE pairs from path and sets them as environment
// variables, but only when the variable is not already set in the environment.
// A missing file is silently ignored; any other error is returned.
func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip blank lines and comments.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("line %d: expected KEY=VALUE, got %q", lineNum, line)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		// Strip optional surrounding quotes (" or ').
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		// Real environment variables take precedence over .env.
		if os.Getenv(key) == "" {
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("line %d: setting %s: %w", lineNum, key, err)
			}
		}
	}
	return scanner.Err()
}
