package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type RefreshToken struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	FamilyID      uuid.UUID
	TokenHash     string
	IssuedAt      time.Time
	ExpiresAt     time.Time
	RevokedAt     *time.Time
	RevokedReason *string
	UserAgent     *string
	IPAddress     *string
}

type LoginAttempt struct {
	Email         string
	IPAddress     string
	UserAgent     string
	Success       bool
	FailureReason string
}

type AuthRepository interface {
	// Refresh tokens
	StoreRefreshToken(ctx context.Context, t *RefreshToken) error
	FindRefreshTokenByHash(ctx context.Context, hash string) (*RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, id uuid.UUID, reason string) error
	RevokeFamily(ctx context.Context, familyID uuid.UUID, reason string) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID, reason string) error
	DeleteExpiredRefreshTokens(ctx context.Context) error

	// Login attempts
	RecordLoginAttempt(ctx context.Context, a *LoginAttempt) error
	CountRecentFailures(ctx context.Context, email string, since time.Time) (int, error)

	// User auth state
	IncrementFailedLogins(ctx context.Context, userID uuid.UUID) (int, error)
	ResetFailedLogins(ctx context.Context, userID uuid.UUID) error
	LockUser(ctx context.Context, userID uuid.UUID, until time.Time) error
	GetUserLockState(ctx context.Context, userID uuid.UUID) (failedCount int, lockedUntil *time.Time, err error)
	MarkEmailVerified(ctx context.Context, userID uuid.UUID) error
	UpdateLastLogin(ctx context.Context, userID uuid.UUID) error

	// Email verifications
	CreateEmailVerification(ctx context.Context, userID uuid.UUID, email, otpHash string, expiresAt time.Time) error
	FindActiveVerification(ctx context.Context, userID uuid.UUID) (otpHash string, expiresAt time.Time, attempts int, err error)
	IncrementVerificationAttempts(ctx context.Context, userID uuid.UUID) error
	ConsumeVerification(ctx context.Context, userID uuid.UUID) error

	// Login OTPs — second-factor codes minted during a 2FA login.
	// Invalidates older outstanding OTPs for the same user on Create.
	CreateLoginOTP(ctx context.Context, userID uuid.UUID, otpHash string, expiresAt time.Time, ip string) error
	FindActiveLoginOTP(ctx context.Context, userID uuid.UUID) (otpHash string, attempts int, err error)
	IncrementLoginOTPAttempts(ctx context.Context, userID uuid.UUID) error
	ConsumeLoginOTP(ctx context.Context, userID uuid.UUID) error

	// Password resets — single-use, expiring tokens delivered via email link.
	CreatePasswordReset(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time, ip, ua string) error
	FindActivePasswordResetByHash(ctx context.Context, tokenHash string) (userID uuid.UUID, err error)
	ConsumePasswordReset(ctx context.Context, tokenHash string) error
	// SetPasswordHash updates a user's password_hash + bumps password_changed_at.
	SetPasswordHash(ctx context.Context, userID uuid.UUID, newHash string) error
}

type authRepository struct {
	db DBTX
}

func NewAuthRepository(db DBTX) AuthRepository {
	return &authRepository{db: db}
}

func (r *authRepository) StoreRefreshToken(ctx context.Context, t *RefreshToken) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO refresh_tokens (id, user_id, family_id, token_hash, issued_at, expires_at, user_agent, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, t.ID, t.UserID, t.FamilyID, t.TokenHash, t.IssuedAt, t.ExpiresAt, t.UserAgent, t.IPAddress)
	return err
}

func (r *authRepository) FindRefreshTokenByHash(ctx context.Context, hash string) (*RefreshToken, error) {
	var t RefreshToken
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, family_id, token_hash, issued_at, expires_at, revoked_at, revoked_reason
		FROM refresh_tokens
		WHERE token_hash = $1
	`, hash).Scan(&t.ID, &t.UserID, &t.FamilyID, &t.TokenHash, &t.IssuedAt, &t.ExpiresAt, &t.RevokedAt, &t.RevokedReason)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

func (r *authRepository) RevokeRefreshToken(ctx context.Context, id uuid.UUID, reason string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = NOW(), revoked_reason = $2
		WHERE id = $1 AND revoked_at IS NULL
	`, id, reason)
	return err
}

func (r *authRepository) RevokeFamily(ctx context.Context, familyID uuid.UUID, reason string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = NOW(), revoked_reason = $2
		WHERE family_id = $1 AND revoked_at IS NULL
	`, familyID, reason)
	return err
}

// RevokeAllForUser revokes every non-revoked refresh token for the user.
// Used by password-change + account-lockout flows so prior sessions can't
// keep refreshing.
func (r *authRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID, reason string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = NOW(), revoked_reason = $2
		WHERE user_id = $1 AND revoked_at IS NULL
	`, userID, reason)
	return err
}

func (r *authRepository) DeleteExpiredRefreshTokens(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `DELETE FROM refresh_tokens WHERE expires_at < NOW() - INTERVAL '7 days'`)
	return err
}

func (r *authRepository) RecordLoginAttempt(ctx context.Context, a *LoginAttempt) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO login_attempts (email, ip_address, user_agent, success, failure_reason)
		VALUES ($1, $2, $3, $4, $5)
	`, a.Email, a.IPAddress, a.UserAgent, a.Success, a.FailureReason)
	return err
}

func (r *authRepository) CountRecentFailures(ctx context.Context, email string, since time.Time) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM login_attempts
		WHERE email = $1 AND success = false AND attempted_at > $2
	`, email, since).Scan(&count)
	return count, err
}

func (r *authRepository) IncrementFailedLogins(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		UPDATE users SET failed_login_count = failed_login_count + 1
		WHERE id = $1 RETURNING failed_login_count
	`, userID).Scan(&count)
	return count, err
}

func (r *authRepository) ResetFailedLogins(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users SET failed_login_count = 0, locked_until = NULL WHERE id = $1
	`, userID)
	return err
}

func (r *authRepository) LockUser(ctx context.Context, userID uuid.UUID, until time.Time) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET locked_until = $2 WHERE id = $1`, userID, until)
	return err
}

func (r *authRepository) GetUserLockState(ctx context.Context, userID uuid.UUID) (int, *time.Time, error) {
	var count int
	var lockedUntil *time.Time
	err := r.db.QueryRow(ctx, `
		SELECT failed_login_count, locked_until FROM users WHERE id = $1
	`, userID).Scan(&count, &lockedUntil)
	return count, lockedUntil, err
}

func (r *authRepository) MarkEmailVerified(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET email_verified = true WHERE id = $1`, userID)
	return err
}

func (r *authRepository) UpdateLastLogin(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET last_login_at = NOW() WHERE id = $1`, userID)
	return err
}

func (r *authRepository) CreateEmailVerification(ctx context.Context, userID uuid.UUID, email, otpHash string, expiresAt time.Time) error {
	// Invalidate any existing active OTPs for this user
	_, err := r.db.Exec(ctx, `
		UPDATE email_verifications SET consumed_at = NOW()
		WHERE user_id = $1 AND consumed_at IS NULL
	`, userID)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO email_verifications (user_id, email, otp_hash, expires_at)
		VALUES ($1, $2, $3, $4)
	`, userID, email, otpHash, expiresAt)
	return err
}

func (r *authRepository) FindActiveVerification(ctx context.Context, userID uuid.UUID) (string, time.Time, int, error) {
	var hash string
	var exp time.Time
	var attempts int
	err := r.db.QueryRow(ctx, `
		SELECT otp_hash, expires_at, attempts FROM email_verifications
		WHERE user_id = $1 AND consumed_at IS NULL
		ORDER BY created_at DESC LIMIT 1
	`, userID).Scan(&hash, &exp, &attempts)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", time.Time{}, 0, nil
		}
		return "", time.Time{}, 0, err
	}
	return hash, exp, attempts, nil
}

func (r *authRepository) IncrementVerificationAttempts(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE email_verifications SET attempts = attempts + 1
		WHERE user_id = $1 AND consumed_at IS NULL
	`, userID)
	return err
}

func (r *authRepository) ConsumeVerification(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE email_verifications SET consumed_at = NOW()
		WHERE user_id = $1 AND consumed_at IS NULL
	`, userID)
	return err
}

// ── Password resets ────────────────────────────────────────────────────────

func (r *authRepository) CreatePasswordReset(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time, ip, ua string) error {
	var ipPtr, uaPtr *string
	if ip != "" {
		ipPtr = &ip
	}
	if ua != "" {
		uaPtr = &ua
	}
	// Invalidate any prior outstanding reset tokens for this user. Each
	// new "forgot password" click must dead-end older emailed links —
	// otherwise an attacker who scrapes a stale inbox link gets a second
	// shot, and the user might reasonably think the latest email is the
	// only valid one.
	if _, err := r.db.Exec(ctx, `
		UPDATE password_resets SET consumed_at = NOW()
		WHERE user_id = $1 AND consumed_at IS NULL
	`, userID); err != nil {
		return err
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO password_resets (id, user_id, token_hash, expires_at, ip_address, user_agent, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
	`, uuid.New(), userID, tokenHash, expiresAt, ipPtr, uaPtr)
	return err
}

// FindActivePasswordResetByHash looks up a non-consumed, non-expired reset
// by its hashed token. Returns the owning user_id.
func (r *authRepository) FindActivePasswordResetByHash(ctx context.Context, tokenHash string) (uuid.UUID, error) {
	var userID uuid.UUID
	err := r.db.QueryRow(ctx, `
		SELECT user_id FROM password_resets
		WHERE token_hash = $1 AND consumed_at IS NULL AND expires_at > NOW()
	`, tokenHash).Scan(&userID)
	return userID, err
}

func (r *authRepository) ConsumePasswordReset(ctx context.Context, tokenHash string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE password_resets SET consumed_at = NOW()
		WHERE token_hash = $1 AND consumed_at IS NULL
	`, tokenHash)
	return err
}

func (r *authRepository) SetPasswordHash(ctx context.Context, userID uuid.UUID, newHash string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users
		SET password_hash = $2, password_changed_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, userID, newHash)
	return err
}

// ── Login OTPs ─────────────────────────────────────────────────────────────

func (r *authRepository) CreateLoginOTP(ctx context.Context, userID uuid.UUID, otpHash string, expiresAt time.Time, ip string) error {
	var ipPtr *string
	if ip != "" {
		ipPtr = &ip
	}
	// Burn any outstanding active OTPs for this user — only one pending
	// code at a time so retries don't leave multiple valid codes.
	if _, err := r.db.Exec(ctx, `
		UPDATE login_otps SET consumed_at = NOW()
		WHERE user_id = $1 AND consumed_at IS NULL
	`, userID); err != nil {
		return err
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO login_otps (id, user_id, otp_hash, expires_at, ip_address, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
	`, uuid.New(), userID, otpHash, expiresAt, ipPtr)
	return err
}

func (r *authRepository) FindActiveLoginOTP(ctx context.Context, userID uuid.UUID) (string, int, error) {
	var hash string
	var attempts int
	err := r.db.QueryRow(ctx, `
		SELECT otp_hash, attempts FROM login_otps
		WHERE user_id = $1 AND consumed_at IS NULL AND expires_at > NOW()
		ORDER BY created_at DESC LIMIT 1
	`, userID).Scan(&hash, &attempts)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", 0, nil
		}
		return "", 0, err
	}
	return hash, attempts, nil
}

func (r *authRepository) IncrementLoginOTPAttempts(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE login_otps SET attempts = attempts + 1
		WHERE user_id = $1 AND consumed_at IS NULL
	`, userID)
	return err
}

func (r *authRepository) ConsumeLoginOTP(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE login_otps SET consumed_at = NOW()
		WHERE user_id = $1 AND consumed_at IS NULL
	`, userID)
	return err
}
