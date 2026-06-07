package domain

import (
	"time"

	"github.com/google/uuid"
)

type Inventory struct {
	ID                uuid.UUID `json:"id" db:"id"`
	ProductID         uuid.UUID `json:"product_id" db:"product_id" validate:"required"`
	Quantity          int       `json:"quantity" db:"quantity" validate:"min=0"`
	LowStockThreshold int       `json:"low_stock_threshold" db:"low_stock_threshold" validate:"min=0"`
	UpdatedAt         time.Time `json:"updated_at" db:"updated_at"`
}
