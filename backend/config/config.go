package config

import (
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	ENV                       string
	Port                      string
	DatabaseURL               string
	ValkeyURL                 string
	JWTSecret                 string
	JWTAccessExpiry           time.Duration
	JWTRefreshExpiry          time.Duration
	AllowedOrigins            string
	RazorpayKeyID             string
	RazorpayKeySecret         string
	RazorpayWebhookSecret     string // primary
	RazorpayWebhookSecretPrev string // previous secret (rotation window)
	RazorpayMockMode          bool
	PayloadEncryptionKey      []byte // 32 bytes (AES-256). Empty → encryption disabled.
	MetricsEnabled            bool
	MetricsToken              string // bearer token for /metrics; empty disables auth
}

func LoadConfig() (*Config, error) {
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

	rzpWebhookSecret := os.Getenv("RAZORPAY_WEBHOOK_SECRET")
	rzpWebhookSecretPrev := os.Getenv("RAZORPAY_WEBHOOK_SECRET_PREV")
	if env == "production" {
		if len(rzpWebhookSecret) < 32 {
			return nil, fmt.Errorf("RAZORPAY_WEBHOOK_SECRET must be set and >= 32 chars in production")
		}
	} else if rzpWebhookSecret == "" {
		rzpWebhookSecret = "mock_webhook_secret_change_in_prod"
	}

	rzpKeyID := firstNonEmpty(os.Getenv("RAZORPAY_KEY_ID"), os.Getenv("RAZORPAY_TEST_API_KEY"))
	rzpKeySecret := firstNonEmpty(os.Getenv("RAZORPAY_KEY_SECRET"), os.Getenv("RAZOR_PAY_SECRET"))

	rzpMock := os.Getenv("RAZORPAY_MOCK_MODE") != "false"
	if !rzpMock && (rzpKeyID == "" || rzpKeySecret == "") {
		rzpMock = true
	}

	// Payload encryption key — 32 bytes hex-encoded. Optional in dev.
	var encKey []byte
	if hexKey := os.Getenv("PAYLOAD_ENCRYPTION_KEY"); hexKey != "" {
		decoded, err := hex.DecodeString(hexKey)
		if err != nil {
			return nil, fmt.Errorf("PAYLOAD_ENCRYPTION_KEY must be hex-encoded: %w", err)
		}
		if len(decoded) != 32 {
			return nil, fmt.Errorf("PAYLOAD_ENCRYPTION_KEY must decode to 32 bytes, got %d", len(decoded))
		}
		encKey = decoded
	} else if env == "production" {
		return nil, fmt.Errorf("PAYLOAD_ENCRYPTION_KEY required in production")
	}

	metricsEnabled := os.Getenv("METRICS_ENABLED") != "false"
	metricsToken := os.Getenv("METRICS_TOKEN")

	return &Config{
		ENV:                       env,
		Port:                      port,
		DatabaseURL:               databaseURL,
		ValkeyURL:                 os.Getenv("VALKEY_URL"),
		JWTSecret:                 jwtSecret,
		JWTAccessExpiry:           accessExpiry,
		JWTRefreshExpiry:          refreshExpiry,
		AllowedOrigins:            allowedOrigins,
		RazorpayKeyID:             rzpKeyID,
		RazorpayKeySecret:         rzpKeySecret,
		RazorpayWebhookSecret:     rzpWebhookSecret,
		RazorpayWebhookSecretPrev: rzpWebhookSecretPrev,
		RazorpayMockMode:          rzpMock,
		PayloadEncryptionKey:      encKey,
		MetricsEnabled:            metricsEnabled,
		MetricsToken:              metricsToken,
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
