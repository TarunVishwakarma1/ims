package shop

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/pkg/sms"
	"github.com/google/uuid"
)

var (
	ErrInvalidPhone = errors.New("invalid phone")
	ErrRateLimited  = errors.New("rate limited")
	ErrInvalidCode  = errors.New("invalid code")
	ErrOTPExpired   = errors.New("otp expired")
	ErrOTPLocked    = errors.New("otp locked")
)

var phoneRe = regexp.MustCompile(`^\+\d{10,15}$`)

// OTPService sends and verifies one-time passwords.
type OTPService interface {
	Send(ctx context.Context, phone, purpose string) (otpID uuid.UUID, expiresInSec int, err error)
	Verify(ctx context.Context, otpID uuid.UUID, code string) (customerID uuid.UUID, token string, err error)
}

type otpService struct {
	otpRepo   repository.OTPRepository
	custRepo  repository.CustomerRepository
	sender    sms.Sender
	jwtSecret string
}

// NewOTPService constructs an OTPService backed by the provided repositories,
// SMS sender, and JWT signing secret.
func NewOTPService(
	otpRepo repository.OTPRepository,
	custRepo repository.CustomerRepository,
	sender sms.Sender,
	jwtSecret string,
) OTPService {
	return &otpService{
		otpRepo:   otpRepo,
		custRepo:  custRepo,
		sender:    sender,
		jwtSecret: jwtSecret,
	}
}

// Send validates the phone, enforces hourly and daily rate limits, generates a
// 6-digit OTP, persists the session, and dispatches the SMS. Returns the
// session ID and expiry in seconds (300).
func (s *otpService) Send(ctx context.Context, phone, purpose string) (uuid.UUID, int, error) {
	if !phoneRe.MatchString(phone) {
		return uuid.Nil, 0, ErrInvalidPhone
	}
	if purpose != domain.OTPPurposeLogin && purpose != domain.OTPPurposeVerify {
		return uuid.Nil, 0, errors.New("invalid purpose")
	}

	now := time.Now().UTC()

	hourCount, err := s.otpRepo.CountSends(ctx, phone, now.Add(-time.Hour))
	if err != nil {
		return uuid.Nil, 0, err
	}
	if hourCount >= domain.OTPSendPerHour {
		return uuid.Nil, 0, ErrRateLimited
	}

	dayCount, err := s.otpRepo.CountSends(ctx, phone, now.Add(-24*time.Hour))
	if err != nil {
		return uuid.Nil, 0, err
	}
	if dayCount >= domain.OTPSendPerDay {
		return uuid.Nil, 0, ErrRateLimited
	}

	code, err := genOTP()
	if err != nil {
		return uuid.Nil, 0, err
	}
	codeHash := sha256Hex(code)

	sess := &domain.OTPSession{
		Phone:     phone,
		CodeHash:  codeHash,
		Purpose:   purpose,
		SentCount: 1,
		ExpiresAt: now.Add(domain.OTPTTL),
	}
	id, err := s.otpRepo.Create(ctx, sess)
	if err != nil {
		return uuid.Nil, 0, err
	}

	if err := s.sender.SendOTP(ctx, phone, code); err != nil {
		return uuid.Nil, 0, fmt.Errorf("sms send: %w", err)
	}

	return id, int(domain.OTPTTL.Seconds()), nil
}

// Verify looks up the OTP session by ID, checks expiry and lockout, compares
// the code, and on success upserts the customer and issues a 30-day JWT.
func (s *otpService) Verify(ctx context.Context, otpID uuid.UUID, code string) (uuid.UUID, string, error) {
	sess, err := s.otpRepo.GetByID(ctx, otpID)
	if err != nil {
		return uuid.Nil, "", err
	}
	if sess == nil {
		return uuid.Nil, "", ErrInvalidCode
	}
	if sess.ConsumedAt != nil {
		return uuid.Nil, "", ErrOTPExpired
	}
	if time.Now().UTC().After(sess.ExpiresAt) {
		return uuid.Nil, "", ErrOTPExpired
	}
	if sess.Attempts >= domain.OTPMaxAttempts {
		return uuid.Nil, "", ErrOTPLocked
	}

	if sha256Hex(code) != sess.CodeHash {
		newAttempts, err := s.otpRepo.IncrementAttempts(ctx, otpID)
		if err != nil {
			return uuid.Nil, "", err
		}
		if newAttempts >= domain.OTPMaxAttempts {
			return uuid.Nil, "", ErrOTPLocked
		}
		return uuid.Nil, "", ErrInvalidCode
	}

	if err := s.otpRepo.MarkConsumed(ctx, otpID); err != nil {
		return uuid.Nil, "", err
	}

	customer, err := s.custRepo.UpsertByPhone(ctx, sess.Phone)
	if err != nil {
		return uuid.Nil, "", err
	}

	token, err := IssueShopJWT(s.jwtSecret, customer.ID, 30*24*time.Hour)
	if err != nil {
		return uuid.Nil, "", err
	}

	return customer.ID, token, nil
}

// genOTP generates a random 6-digit numeric OTP using crypto/rand.
func genOTP() (string, error) {
	const digits = "0123456789"
	b := make([]byte, domain.OTPCodeLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = digits[int(b[i])%10]
	}
	return string(b), nil
}

// sha256Hex returns the lowercase hex-encoded SHA-256 hash of s.
func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
