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
	PaymentStatus          string          `json:"payment_status"`
	PaymentID              *string         `json:"payment_id"`
	AcceptedAt             *time.Time      `json:"accepted_at"`
	ShippedAt              *time.Time      `json:"shipped_at"`
	DeliveredAt            *time.Time      `json:"delivered_at"`
	CompletedAt            *time.Time      `json:"completed_at"`
	CancelledAt            *time.Time      `json:"cancelled_at"`
}

type OrderItem struct {
	ID        uuid.UUID `json:"id" db:"id"`
	OrgID     uuid.UUID `json:"org_id" db:"org_id"`
	OrderID   uuid.UUID `json:"order_id" db:"order_id" validate:"required"`
	ProductID uuid.UUID `json:"product_id" db:"product_id" validate:"required"`
	Quantity  int       `json:"quantity" db:"quantity" validate:"min=1"`
	UnitPrice int64     `json:"unit_price" db:"unit_price"`
}
