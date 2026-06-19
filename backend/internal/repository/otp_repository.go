package repository

import (
	"context"
	"errors"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OTPRepository handles persistence for otp_sessions.
type OTPRepository interface {
	Create(ctx context.Context, s *domain.OTPSession) (uuid.UUID, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.OTPSession, error)
	MarkConsumed(ctx context.Context, id uuid.UUID) error
	IncrementAttempts(ctx context.Context, id uuid.UUID) (int, error)
	CountSends(ctx context.Context, phone string, since time.Time) (int, error)
}

type otpRepository struct{ pool *pgxpool.Pool }

// NewOTPRepository returns an OTPRepository backed by pool.
func NewOTPRepository(pool *pgxpool.Pool) OTPRepository {
	return &otpRepository{pool: pool}
}

// Create inserts a new OTP session and returns its generated ID.
func (r *otpRepository) Create(ctx context.Context, s *domain.OTPSession) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `
		INSERT INTO otp_sessions (phone, code_hash, purpose, sent_count, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, s.Phone, s.CodeHash, s.Purpose, s.SentCount, s.ExpiresAt).Scan(&id)
	return id, err
}

// GetByID returns the OTP session with the given ID, or (nil, nil) if not found.
func (r *otpRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.OTPSession, error) {
	s := &domain.OTPSession{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, phone, code_hash, purpose, attempts, sent_count, expires_at, consumed_at, created_at
		FROM otp_sessions
		WHERE id = $1
	`, id).Scan(
		&s.ID, &s.Phone, &s.CodeHash, &s.Purpose,
		&s.Attempts, &s.SentCount,
		&s.ExpiresAt, &s.ConsumedAt, &s.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

// MarkConsumed sets consumed_at to NOW() for the session. Idempotent: a second
// call on an already-consumed session is a no-op and returns no error.
func (r *otpRepository) MarkConsumed(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE otp_sessions SET consumed_at = NOW() WHERE id = $1 AND consumed_at IS NULL`,
		id,
	)
	return err
}

// IncrementAttempts increments the attempts counter by 1 and returns the new value.
func (r *otpRepository) IncrementAttempts(ctx context.Context, id uuid.UUID) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		`UPDATE otp_sessions SET attempts = attempts + 1 WHERE id = $1 RETURNING attempts`,
		id,
	).Scan(&n)
	return n, err
}

// CountSends counts how many OTP sessions exist for phone created at or after since.
func (r *otpRepository) CountSends(ctx context.Context, phone string, since time.Time) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM otp_sessions WHERE phone = $1 AND created_at >= $2`,
		phone, since,
	).Scan(&n)
	return n, err
}
