package domain

import (
	"time"

	"github.com/google/uuid"
)

type AuditLog struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	UserID    *uuid.UUID `json:"user_id" db:"user_id"`
	Action    string     `json:"action" db:"action"`
	Entity    string     `json:"entity" db:"entity"`
	EntityID  uuid.UUID  `json:"entity_id" db:"entity_id"`
	IPAddress string     `json:"ip_address" db:"ip_address"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
}
