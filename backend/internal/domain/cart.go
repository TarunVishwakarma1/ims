package domain

import (
	"time"

	"github.com/google/uuid"
)

type Cart struct {
	CustomerID uuid.UUID  `json:"customer_id"`
	UpdatedAt  time.Time  `json:"updated_at"`
	Items      []CartItem `json:"items"`
}

type CartItem struct {
	ProductID      uuid.UUID `json:"product_id"`
	Qty            int       `json:"qty"`
	UnitPricePaise int64     `json:"unit_price_paise"`
	AddedAt        time.Time `json:"added_at"`
}
