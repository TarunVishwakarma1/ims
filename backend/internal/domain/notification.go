package domain

import (
	"time"

	"github.com/google/uuid"
)

type NotificationStatus string

const (
	NotificationStatusPending NotificationStatus = "pending"
	NotificationStatusSent    NotificationStatus = "sent"
	NotificationStatusFailed  NotificationStatus = "failed"
	NotificationStatusDLQ     NotificationStatus = "dlq"
)

// Notification is a queued outbound message. Persisted before send so it
// survives crashes; a background worker drives status transitions.
type Notification struct {
	ID          uuid.UUID          `json:"id"`
	Channel     string             `json:"channel"` // email | sms | push
	Recipient   string             `json:"recipient"`
	Subject     string             `json:"subject"`
	BodyText    string             `json:"body_text"`
	BodyHTML    *string            `json:"body_html,omitempty"`
	Status      NotificationStatus `json:"status"`
	Attempts    int                `json:"attempts"`
	LastError   *string            `json:"last_error,omitempty"`
	NextAttempt time.Time          `json:"next_attempt"`
	CreatedAt   time.Time          `json:"created_at"`
	SentAt      *time.Time         `json:"sent_at,omitempty"`
}
