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
	RazorpayWebhookSecret     string // primary (test or live, picked by RazorpayLiveMode)
	RazorpayWebhookSecretPrev string // previous secret (rotation window)
	RazorpayMockMode          bool
	RazorpayLiveMode          bool   // true → real money. False → RazorPay test sandbox.
	PayloadEncryptionKey      []byte // 32 bytes (AES-256). Empty → encryption disabled.
	MetricsEnabled            bool
	MetricsToken              string // bearer token for /metrics; empty disables auth

	// SMTP — empty SMTPHost → fall back to log-only emailer (dev default).
	SMTPHost      string
	SMTPPort      string
	SMTPUsername  string
	SMTPPassword  string
	SMTPFromEmail string // e.g. "no-reply@example.com"
	SMTPFromName  string // e.g. "IMS Notifications"
	WebAppURL     string // for action links in emails (e.g. https://app.example.com)

	// B2C Shop
	ShopEnabled     bool
	ShopOrgID       string
	MSG91AuthKey    string
	MSG91TemplateID string
	MSG91SenderID   string
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

	// Razorpay mode resolution. Three orthogonal flags:
	//   RAZORPAY_MOCK_MODE=true  → use the internal mock (no external calls)
	//   RAZORPAY_LIVE_MODE=true  → use real-money live keys/webhook
	//   neither                  → use test sandbox (default)
	//
	// Mock wins if both mock and live are set — safer default.
	rzpMock := os.Getenv("RAZORPAY_MOCK_MODE") == "true"
	rzpLive := os.Getenv("RAZORPAY_LIVE_MODE") == "true"

	var rzpKeyID, rzpKeySecret, rzpWebhookSecret string
	if rzpLive && !rzpMock {
		rzpKeyID = firstNonEmpty(os.Getenv("RAZORPAY_LIVE_API_KEY"), os.Getenv("RAZORPAY_KEY_ID"))
		rzpKeySecret = firstNonEmpty(os.Getenv("RAZOR_PAY_LIVE_SECRET"), os.Getenv("RAZORPAY_LIVE_KEY_SECRET"), os.Getenv("RAZORPAY_KEY_SECRET"))
		rzpWebhookSecret = firstNonEmpty(os.Getenv("RAZORPAY_WEBHOOK_SECRET_LIVE"), os.Getenv("RAZORPAY_WEBHOOK_SECRET"))
	} else {
		rzpKeyID = firstNonEmpty(os.Getenv("RAZORPAY_TEST_API_KEY"), os.Getenv("RAZORPAY_KEY_ID"))
		rzpKeySecret = firstNonEmpty(os.Getenv("RAZOR_PAY_SECRET"), os.Getenv("RAZOR_PAY_TEST_SECRET"), os.Getenv("RAZORPAY_KEY_SECRET"))
		rzpWebhookSecret = firstNonEmpty(os.Getenv("RAZORPAY_WEBHOOK_SECRET_TEST"), os.Getenv("RAZORPAY_WEBHOOK_SECRET"))
	}
	rzpWebhookSecretPrev := os.Getenv("RAZORPAY_WEBHOOK_SECRET_PREV")

	// Live mode in production must have a real webhook secret.
	if rzpLive && env == "production" && len(rzpWebhookSecret) < 32 {
		return nil, fmt.Errorf("RAZORPAY_WEBHOOK_SECRET_LIVE must be set and >= 32 chars when live in production")
	}
	if rzpWebhookSecret == "" {
		rzpWebhookSecret = "mock_webhook_secret_change_in_prod"
	}

	// Fall back to mock if credentials are missing for the selected mode.
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

	shopEnabled := os.Getenv("SHOP_ENABLED") == "true"
	msg91SenderID := os.Getenv("MSG91_SENDER_ID")
	if msg91SenderID == "" {
		msg91SenderID = "IMSHOP"
	}

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
		RazorpayLiveMode:          rzpLive && !rzpMock,
		PayloadEncryptionKey:      encKey,
		MetricsEnabled:            metricsEnabled,
		MetricsToken:              metricsToken,
		SMTPHost:                  os.Getenv("SMTP_HOST"),
		SMTPPort:                  os.Getenv("SMTP_PORT"),
		SMTPUsername:              os.Getenv("SMTP_USERNAME"),
		SMTPPassword:              os.Getenv("SMTP_PASSWORD"),
		SMTPFromEmail:             os.Getenv("SMTP_FROM_EMAIL"),
		SMTPFromName:              os.Getenv("SMTP_FROM_NAME"),
		WebAppURL:                 os.Getenv("WEB_APP_URL"),
		ShopEnabled:               shopEnabled,
		ShopOrgID:                 os.Getenv("SHOP_ORG_ID"),
		MSG91AuthKey:              os.Getenv("MSG91_AUTH_KEY"),
		MSG91TemplateID:           os.Getenv("MSG91_TEMPLATE_ID"),
		MSG91SenderID:             msg91SenderID,
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
