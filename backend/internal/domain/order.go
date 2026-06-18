package domain

import (
	"time"

	"github.com/google/uuid"
)

type OrderStatus string

const (
	OrderStatusPending    OrderStatus = "pending"
	OrderStatusConfirmed  OrderStatus = "confirmed"
	OrderStatusAccepted   OrderStatus = "accepted"
	OrderStatusRejected   OrderStatus = "rejected"
	OrderStatusProcessing OrderStatus = "processing"
	OrderStatusReady      OrderStatus = "ready"
	OrderStatusShipped    OrderStatus = "shipped"
	OrderStatusDelivered  OrderStatus = "delivered"
	OrderStatusCompleted  OrderStatus = "completed"
	OrderStatusCancelled  OrderStatus = "cancelled"
	OrderStatusRefunded   OrderStatus = "refunded"
)

type Order struct {
	ID                     uuid.UUID       `json:"id" db:"id"`
	OrgID                  uuid.UUID       `json:"org_id" db:"org_id"`
	UserID                 uuid.UUID       `json:"user_id" db:"user_id" validate:"required"`
	Status                 OrderStatus     `json:"status" db:"status" validate:"required,oneof=pending confirmed accepted rejected processing ready shipped delivered completed cancelled refunded"`
	TotalAmount            int64           `json:"total_amount" db:"total_amount"`
	CreatedAt              time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt              time.Time       `json:"updated_at" db:"updated_at"`
	OrderType              string          `json:"order_type"`
	BuyerOrgID             *uuid.UUID      `json:"buyer_org_id"`
	SupplierOrgID          *uuid.UUID      `json:"supplier_org_id"`
	SupplierLocationID     *uuid.UUID      `json:"supplier_location_id"`
	CustomerID             *uuid.UUID      `json:"customer_id"`
	DeliveryAddressID      *uuid.UUID      `json:"delivery_address_id"`
	DeliveryAddressSnapshot *map[string]any `json:"delivery_address_snapshot"`
	Subtotal               int64           `json:"subtotal"`
	DeliveryFee            int64           `json:"delivery_fee"`
	Discount               int64           `json:"discount"`
	TaxAmount              int64           `json:"tax_amount"`
	TaxCGST                int64           `json:"tax_cgst"`
	TaxSGST                int64           `json:"tax_sgst"`
	TaxIGST                int64           `json:"tax_igst"`
	IsInterState           bool            `json:"is_inter_state"`
	PaymentStatus          string          `json:"payment_status"`
	PaymentID              *string         `json:"payment_id"`
	AcceptedAt             *time.Time      `json:"accepted_at"`
	ShippedAt              *time.Time      `json:"shipped_at"`
	DeliveredAt            *time.Time      `json:"delivered_at"`
	CompletedAt            *time.Time      `json:"completed_at"`
	CancelledAt            *time.Time      `json:"cancelled_at"`

	// Optional display names — populated only by endpoints that JOIN the
	// organizations table (partner detail recent-orders). Empty for normal
	// list/get paths; UI falls back to UUID prefix when blank.
	BuyerOrgName    string `json:"buyer_org_name,omitempty"`
	SupplierOrgName string `json:"supplier_org_name,omitempty"`
}

// OrderListFilters captures every supported list / search / export
// filter for orders. Empty/zero values mean "no constraint".
type OrderListFilters struct {
	Status        string     // exact match, e.g. "pending"
	PaymentStatus string     // exact match, e.g. "paid"
	OrderType     string     // exact match, e.g. "b2b"
	Search        string     // free-text — matches order id prefix / user id prefix
	From          *time.Time // inclusive lower bound on created_at
	To            *time.Time // inclusive upper bound on created_at
	Page          int        // 1-indexed; 0 → 1
	PerPage       int        // 0 → 25, clamped to [1,200]
}

// OrderListResult bundles a page of orders with the unfiltered total
// (used by the UI to render "showing N of M" + pagination).
type OrderListResult struct {
	Items   []*Order `json:"items"`
	Total   int      `json:"total"`
	Page    int      `json:"page"`
	PerPage int      `json:"per_page"`
}

type OrderItem struct {
	ID        uuid.UUID `json:"id" db:"id"`
	OrgID     uuid.UUID `json:"org_id" db:"org_id"`
	OrderID   uuid.UUID `json:"order_id" db:"order_id" validate:"required"`
	ProductID uuid.UUID `json:"product_id" db:"product_id" validate:"required"`
	Quantity  int       `json:"quantity" db:"quantity" validate:"min=1"`
	UnitPrice int64     `json:"unit_price" db:"unit_price"`
}
