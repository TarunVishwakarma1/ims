package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Payment status values mirror the lifecycle of a real RazorPay transaction.
const (
	PaymentStatusCreated            = "created"
	PaymentStatusAuthorized         = "authorized"
	PaymentStatusCaptured           = "captured"
	PaymentStatusFailed             = "failed"
	PaymentStatusPartiallyRefunded  = "partially_refunded"
	PaymentStatusRefunded           = "refunded"
)

type Payment struct {
	ID                uuid.UUID       `json:"id"`
	OrgID             uuid.UUID       `json:"org_id"`
	OrderID           *uuid.UUID      `json:"order_id"`
	RazorpayOrderID   string          `json:"razorpay_order_id"`
	RazorpayPaymentID *string         `json:"razorpay_payment_id"`
	Amount            int64           `json:"amount"`           // paise
	AmountRefunded    int64           `json:"amount_refunded"`  // paise — cumulative across refunds
	Currency          string          `json:"currency"`         // INR
	Status            string          `json:"status"`
	Method            *string         `json:"method"`
	FailureReason     *string         `json:"failure_reason"`
	RawPayload        json.RawMessage `json:"raw_payload,omitempty"`
	IsMock            bool            `json:"is_mock"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

// Refund is one entry in the refund history for a payment. Multiple refunds
// can exist per payment for partial refund cases.
type Refund struct {
	ID               uuid.UUID `json:"id"`
	PaymentID        uuid.UUID `json:"payment_id"`
	OrgID            uuid.UUID `json:"org_id"`
	Amount           int64     `json:"amount"` // paise
	RazorpayRefundID *string   `json:"razorpay_refund_id"`
	Status           string    `json:"status"` // processed | failed | pending
	Reason           *string   `json:"reason"`
	CreatedAt        time.Time `json:"created_at"`
}

// WebhookEvent stores every received webhook so we can deduplicate, audit, and
// replay if something blew up. `event_id` is provider-supplied and unique.
type WebhookEvent struct {
	ID          uuid.UUID       `json:"id"`
	Provider    string          `json:"provider"`
	EventID     string          `json:"event_id"`
	EventType   string          `json:"event_type"`
	Signature   *string         `json:"signature,omitempty"`
	Payload     json.RawMessage `json:"payload"`
	Status      string          `json:"status"`
	Error       *string         `json:"error,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	ProcessedAt *time.Time      `json:"processed_at,omitempty"`
}
