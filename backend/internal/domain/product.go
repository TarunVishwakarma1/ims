package domain

import (
	"time"

	"github.com/google/uuid"
)

type Product struct {
	ID          uuid.UUID `json:"id" db:"id"`
	OrgID       uuid.UUID `json:"org_id" db:"org_id"`
	CategoryID  uuid.UUID `json:"category_id" db:"category_id" validate:"required"`
	Name        string    `json:"name" db:"name" validate:"required"`
	Description string    `json:"description" db:"description"`
	SKU         string    `json:"sku" db:"sku" validate:"required"`
	Price       int64     `json:"price" db:"price"`
	GSTRate     int       `json:"gst_rate" db:"gst_rate"` // percent (0..28); 0 = exempt
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}
