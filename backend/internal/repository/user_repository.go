package repository

import (
	"context"
	"errors"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
	Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error
	List(ctx context.Context, orgID uuid.UUID) ([]*domain.User, error)

	// SetTOTP writes the TOTP secret / enabled flag / backup codes / verified
	// timestamp in a single update. Used by the 2FA enroll + confirm flow.
	SetTOTP(ctx context.Context, userID uuid.UUID, secret *string, enabled bool, verifiedAt *time.Time, backupCodes *string) error

	// SetEmail2FA toggles the email-based second factor flag.
	SetEmail2FA(ctx context.Context, userID uuid.UUID, enabled bool) error

	WithTx(tx pgx.Tx) UserRepository
}

type userRepository struct {
	db DBTX
}

func NewUserRepository(db DBTX) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) WithTx(tx pgx.Tx) UserRepository {
	return &userRepository{db: tx}
}

func (r *userRepository) Create(ctx context.Context, user *domain.User) error {
	query := `
		INSERT INTO users (id, org_id, name, email, password_hash, role, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.Exec(ctx, query, user.ID, user.OrgID, user.Name, user.Email, user.PasswordHash, user.Role, user.IsActive, user.CreatedAt, user.UpdatedAt)
	return err
}

const userSelectCols = `id, org_id, name, email, password_hash, role, is_active,
	email_verified, failed_login_count, locked_until, last_login_at, password_changed_at,
	created_at, updated_at,
	totp_secret, totp_enabled, totp_verified_at, totp_backup_codes,
	email_2fa_enabled`

func scanUser(scanner interface {
	Scan(dest ...any) error
}) (*domain.User, error) {
	u := &domain.User{}
	err := scanner.Scan(&u.ID, &u.OrgID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.IsActive,
		&u.EmailVerified, &u.FailedLoginCount, &u.LockedUntil, &u.LastLoginAt, &u.PasswordChangedAt,
		&u.CreatedAt, &u.UpdatedAt,
		&u.TOTPSecret, &u.TOTPEnabled, &u.TOTPVerifiedAt, &u.TOTPBackupCodes,
		&u.EmailTwoFAEnabled)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *userRepository) GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*domain.User, error) {
	// orgID == uuid.Nil → cross-org lookup (used by refresh token flow to load by ID alone).
	var query string
	var args []any
	if orgID == uuid.Nil {
		query = `SELECT ` + userSelectCols + ` FROM users WHERE id = $1`
		args = []any{id}
	} else {
		query = `SELECT ` + userSelectCols + ` FROM users WHERE id = $1 AND org_id = $2`
		args = []any{id, orgID}
	}

	user, err := scanUser(r.db.QueryRow(ctx, query, args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return user, nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `SELECT ` + userSelectCols + ` FROM users WHERE email = $1`
	user, err := scanUser(r.db.QueryRow(ctx, query, email))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return user, nil
}

func (r *userRepository) SetTOTP(ctx context.Context, userID uuid.UUID, secret *string, enabled bool, verifiedAt *time.Time, backupCodes *string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users SET
			totp_secret = $2,
			totp_enabled = $3,
			totp_verified_at = $4,
			totp_backup_codes = $5,
			updated_at = NOW()
		WHERE id = $1
	`, userID, secret, enabled, verifiedAt, backupCodes)
	return err
}

func (r *userRepository) SetEmail2FA(ctx context.Context, userID uuid.UUID, enabled bool) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users SET email_2fa_enabled = $2, updated_at = NOW() WHERE id = $1
	`, userID, enabled)
	return err
}

func (r *userRepository) Update(ctx context.Context, user *domain.User) error {
	query := `
		UPDATE users
		SET name = $3, email = $4, password_hash = $5, role = $6, is_active = $7, updated_at = $8
		WHERE id = $1 AND org_id = $2
	`
	_, err := r.db.Exec(ctx, query, user.ID, user.OrgID, user.Name, user.Email, user.PasswordHash, user.Role, user.IsActive, user.UpdatedAt)
	return err
}

func (r *userRepository) Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error {
	query := `
		UPDATE users
		SET is_active = false, updated_at = NOW()
		WHERE id = $1 AND org_id = $2
	`
	_, err := r.db.Exec(ctx, query, id, orgID)
	return err
}

func (r *userRepository) List(ctx context.Context, orgID uuid.UUID) ([]*domain.User, error) {
	query := `SELECT ` + userSelectCols + ` FROM users WHERE org_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]*domain.User, 0)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return users, nil
}
