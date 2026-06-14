package config

import (
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	ENV              string
	Port             string
	DatabaseURL      string
	ValkeyURL        string // e.g. redis://valkey:6379/0 — optional, app runs without it
	JWTSecret        string
	JWTAccessExpiry  time.Duration
	JWTRefreshExpiry time.Duration
	AllowedOrigins   string
}

func LoadConfig() (*Config, error) {

	// Try to load .env.docker or .env. Ignore the error if they don't exist
	// because in production we will use native OS env vars (k8s/docker secrets)
	_ = godotenv.Load(".env.docker", ".env")

	env := os.Getenv("ENV")
	if env != "production" && env != "development" && env != "testing" {
		return nil, fmt.Errorf("invalid ENV value %q: must be development, production, or testing", env)
	}

	port := os.Getenv("PORT")
	if port == "" {
		return nil, fmt.Errorf("port is not set")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, fmt.Errorf("database URL is not set")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" || len(jwtSecret) < 32 {
		return nil, fmt.Errorf("JWT secret is not set or less than 32 characters")
	}

	accessExpiry, err := time.ParseDuration(os.Getenv("JWT_ACCESS_EXPIRY"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_ACCESS_EXPIRY: %w", err)
	}

	refreshExpiry, err := time.ParseDuration(os.Getenv("JWT_REFRESH_EXPIRY"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_REFRESH_EXPIRY: %w", err)
	}

	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		return nil, fmt.Errorf("ALLOWED_ORIGINS is not set")
	}

	return &Config{
		ENV:              env,
		Port:             port,
		DatabaseURL:      databaseURL,
		ValkeyURL:        os.Getenv("VALKEY_URL"),
		JWTSecret:        jwtSecret,
		JWTAccessExpiry:  accessExpiry,
		JWTRefreshExpiry: refreshExpiry,
		AllowedOrigins:   allowedOrigins,
	}, nil
}
