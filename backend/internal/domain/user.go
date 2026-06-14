package domain

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID                uuid.UUID  `json:"id" db:"id"`
	OrgID             uuid.UUID  `json:"org_id" db:"org_id"`
	Name              string     `json:"name" db:"name" validate:"required"`
	Email             string     `json:"email" db:"email" validate:"required,email"`
	PasswordHash      string     `json:"-" db:"password_hash" validate:"required"`
	Role              string     `json:"role" db:"role" validate:"required"`
	IsActive          bool       `json:"is_active" db:"is_active"`
	EmailVerified     bool       `json:"email_verified" db:"email_verified"`
	FailedLoginCount  int        `json:"-" db:"failed_login_count"`
	LockedUntil       *time.Time `json:"-" db:"locked_until"`
	LastLoginAt       *time.Time `json:"last_login_at,omitempty" db:"last_login_at"`
	PasswordChangedAt *time.Time `json:"-" db:"password_changed_at"`
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at" db:"updated_at"`
}


