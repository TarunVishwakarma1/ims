package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	OTPPurposeLogin  = "login"
	OTPPurposeVerify = "verify"

	OTPCodeLen       = 6
	OTPTTL           = 5 * time.Minute
	OTPMaxAttempts   = 5
	OTPSendPerHour   = 5
	OTPSendPerDay    = 20
	OTPLockoutWindow = 15 * time.Minute
)

type OTPSession struct {
	ID         uuid.UUID
	Phone      string
	CodeHash   string
	Purpose    string
	Attempts   int
	SentCount  int
	ExpiresAt  time.Time
	ConsumedAt *time.Time
	CreatedAt  time.Time
}
