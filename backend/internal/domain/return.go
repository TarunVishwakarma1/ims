package domain

import (
	"time"

	"github.com/google/uuid"
)

type ReturnStatus string

const (
	ReturnStatusRequested ReturnStatus = "requested"
	ReturnStatusApproved  ReturnStatus = "approved"
	ReturnStatusRejected  ReturnStatus = "rejected"
	ReturnStatusInTransit ReturnStatus = "in_transit"
	ReturnStatusReceived  ReturnStatus = "received"
	ReturnStatusRefunded  ReturnStatus = "refunded"
)

// ReturnRequest is one RMA case for an order. Per-item quantities allow
// partial returns (return 2 of 3 ordered units). refund_amount is computed
// at request time and frozen — later status changes don't recompute.
type ReturnRequest struct {
	ID            uuid.UUID    `json:"id"`
	OrgID         uuid.UUID    `json:"org_id"`
	OrderID       uuid.UUID    `json:"order_id"`
	RequestedBy   uuid.UUID    `json:"requested_by"`
	Status        ReturnStatus `json:"status"`
	Reason        string       `json:"reason"`
	RefundAmount  int64        `json:"refund_amount"`
	RejectionNote *string      `json:"rejection_note,omitempty"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
	ApprovedAt    *time.Time   `json:"approved_at,omitempty"`
	ReceivedAt    *time.Time   `json:"received_at,omitempty"`
	RefundedAt    *time.Time   `json:"refunded_at,omitempty"`

	// Direction relative to the requesting org. "outgoing" → I'm the buyer
	// returning items to a supplier. "incoming" → I'm the supplier and a
	// buyer is returning items to me. Set by repository layer based on
	// the caller's org id; not persisted.
	Direction string `json:"direction,omitempty"`

	// Items optionally populated by GetByID for the detail view.
	Items []*ReturnItem `json:"items,omitempty"`
}

type ReturnItem struct {
	ID          uuid.UUID `json:"id"`
	ReturnID    uuid.UUID `json:"return_id"`
	OrderItemID uuid.UUID `json:"order_item_id"`
	ProductID   uuid.UUID `json:"product_id"`
	Quantity    int       `json:"quantity"`
	UnitPrice   int64     `json:"unit_price"`
}

// CanTransitionReturn enforces the return state machine. Refused
// transitions return false; the service maps that to ErrConflict.
func CanTransitionReturn(from, to ReturnStatus) bool {
	switch from {
	case ReturnStatusRequested:
		return to == ReturnStatusApproved || to == ReturnStatusRejected
	case ReturnStatusApproved:
		return to == ReturnStatusInTransit || to == ReturnStatusRejected
	case ReturnStatusInTransit:
		return to == ReturnStatusReceived
	case ReturnStatusReceived:
		return to == ReturnStatusRefunded
	}
	return false
}
