package service

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// TOTPService manages two-factor enrollment + verification. Stateless;
// reads/writes go through UserRepository so a user's TOTP state moves with
// their normal row (no separate table).
type TOTPService interface {
	// Enroll generates a fresh secret + provisioning URI for a user.
	// The secret is persisted but `enabled` stays false until the user
	// confirms by submitting a valid code via Confirm.
	Enroll(ctx context.Context, userID uuid.UUID, accountName string) (uri string, secret string, err error)

	// Confirm verifies the supplied 6-digit code against the pending
	// secret. On success: flips enabled=true, stamps verified_at, and
	// returns one-time backup codes that the UI must show ONCE.
	Confirm(ctx context.Context, userID uuid.UUID, code string) (backupCodes []string, err error)

	// Disable clears the secret + flag. Requires either a valid TOTP
	// code (re-prove) or admin context — enforce upstream.
	Disable(ctx context.Context, userID uuid.UUID) error

	// Validate runs at login time. Accepts either a current TOTP code OR
	// a single-use backup code. Backup codes are consumed on use.
	Validate(ctx context.Context, user *domain.User, code string) error
}

type totpService struct {
	users repository.UserRepository
}

func NewTOTPService(users repository.UserRepository) TOTPService {
	return &totpService{users: users}
}

const issuer = "IMS"

func (s *totpService) Enroll(ctx context.Context, userID uuid.UUID, accountName string) (string, string, error) {
	// Pin period/digits/algorithm so the QR encodes them explicitly.
	// Match Validate's expectations exactly — protects against future
	// library-default changes.
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountName,
		Period:      30,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return "", "", err
	}
	secret := key.Secret()
	// Persist secret but leave enabled=false until Confirm.
	if err := s.users.SetTOTP(ctx, userID, &secret, false, nil, nil); err != nil {
		return "", "", err
	}
	return key.URL(), secret, nil
}

func (s *totpService) Confirm(ctx context.Context, userID uuid.UUID, code string) ([]string, error) {
	u, err := s.users.GetByID(ctx, userID, uuid.Nil)
	if err != nil {
		return nil, err
	}
	if u.TOTPSecret == nil {
		return nil, errors.New("no pending enrollment — call Enroll first")
	}
	// Same tolerance window as login (Validate): accept ±30s drift so a
	// container with slightly stale clock vs the authenticator doesn't
	// reject correct codes. Six digits / SHA1 / 30s period — matches
	// every consumer authenticator app.
	valid, vErr := totp.ValidateCustom(code, *u.TOTPSecret, time.Now(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if vErr != nil || !valid {
		return nil, errors.New("invalid code")
	}
	codes, err := generateBackupCodes(8)
	if err != nil {
		return nil, err
	}
	stored := strings.Join(codes, ",")
	now := time.Now().UTC()
	if err := s.users.SetTOTP(ctx, userID, u.TOTPSecret, true, &now, &stored); err != nil {
		return nil, err
	}
	return codes, nil
}

func (s *totpService) Disable(ctx context.Context, userID uuid.UUID) error {
	return s.users.SetTOTP(ctx, userID, nil, false, nil, nil)
}

func (s *totpService) Validate(ctx context.Context, u *domain.User, code string) error {
	if !u.TOTPEnabled || u.TOTPSecret == nil {
		return errors.New("2FA not enabled for this user")
	}
	// Try TOTP first.
	valid, err := totp.ValidateCustom(code, *u.TOTPSecret, time.Now(), totp.ValidateOpts{
		Period:    30,
		Skew:      1, // accept ±30s drift
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err == nil && valid {
		return nil
	}
	// Fallback: backup code. Consume on use.
	if u.TOTPBackupCodes == nil || *u.TOTPBackupCodes == "" {
		return errors.New("invalid code")
	}
	codes := strings.Split(*u.TOTPBackupCodes, ",")
	kept := make([]string, 0, len(codes))
	consumed := false
	for _, c := range codes {
		if c == code && !consumed {
			consumed = true
			continue
		}
		kept = append(kept, c)
	}
	if !consumed {
		return errors.New("invalid code")
	}
	joined := strings.Join(kept, ",")
	return s.users.SetTOTP(ctx, u.ID, u.TOTPSecret, true, u.TOTPVerifiedAt, &joined)
}

// generateBackupCodes returns N random base32 codes of fixed length.
// Codes are presented to the user once; they don't repeat across sessions.
func generateBackupCodes(n int) ([]string, error) {
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		buf := make([]byte, 8)
		if _, err := rand.Read(buf); err != nil {
			return nil, err
		}
		raw := strings.TrimRight(base32.StdEncoding.EncodeToString(buf), "=")
		// 12 chars, dashed for readability: XXXX-XXXX-XXXX
		out = append(out, fmt.Sprintf("%s-%s-%s", raw[0:4], raw[4:8], raw[8:12]))
	}
	return out, nil
}
