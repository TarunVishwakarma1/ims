package domain

import (
	"time"

	"github.com/google/uuid"
)

type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusConfirmed OrderStatus = "confirmed"
	OrderStatusCancelled OrderStatus = "cancelled"
)

type Order struct {
	ID          uuid.UUID   `json:"id" db:"id"`
	UserID      uuid.UUID   `json:"user_id" db:"user_id" validate:"required"`
	Status      OrderStatus `json:"status" db:"status" validate:"required,oneof=pending confirmed cancelled"`
	TotalAmount int64       `json:"total_amount" db:"total_amount"`
	CreatedAt   time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at" db:"updated_at"`
}

type OrderItem struct {
	ID        uuid.UUID `json:"id" db:"id"`
	OrderID   uuid.UUID `json:"order_id" db:"order_id" validate:"required"`
	ProductID uuid.UUID `json:"product_id" db:"product_id" validate:"required"`
	Quantity  int       `json:"quantity" db:"quantity" validate:"min=1"`
	UnitPrice int64     `json:"unit_price" db:"unit_price"`
}
