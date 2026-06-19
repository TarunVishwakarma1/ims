package domain

import (
	"time"

	"github.com/google/uuid"
)

type MarketplaceListing struct {
	ID           uuid.UUID  `json:"id"`
	OrgID        uuid.UUID  `json:"org_id"`
	ProductID    uuid.UUID  `json:"product_id"`
	LocationID   *uuid.UUID `json:"location_id"`
	ListingPrice int64      `json:"listing_price"` // paise
	MinOrderQty  int        `json:"min_order_qty"`
	MaxOrderQty  *int       `json:"max_order_qty"` // nullable
	IsActive     bool       `json:"is_active"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`

	// Joined fields for search results
	ProductName   string   `json:"product_name,omitempty"`
	ProductSKU    string   `json:"product_sku,omitempty"`
	OrgName       string   `json:"org_name,omitempty"`
	OrgSlug       string   `json:"org_slug,omitempty"`
	LocationName  string   `json:"location_name,omitempty"`
	LocationCity  string   `json:"location_city,omitempty"`
	LocationLat   *float64 `json:"location_lat,omitempty"`
	LocationLng   *float64 `json:"location_lng,omitempty"`
	StockQuantity int      `json:"stock_quantity,omitempty"`
	DistanceKM    *float64 `json:"distance_km,omitempty"`
}

type MarketplaceCart struct {
	ID         uuid.UUID  `json:"id"`
	BuyerOrgID *uuid.UUID `json:"buyer_org_id"`
	CustomerID *uuid.UUID `json:"customer_id"`
	ExpiresAt  time.Time  `json:"expires_at"`
	CreatedAt  time.Time  `json:"created_at"`
	Items      []MarketplaceCartItem `json:"items,omitempty"`
}

type MarketplaceCartItem struct {
	ID        uuid.UUID           `json:"id"`
	CartID    uuid.UUID           `json:"cart_id"`
	ListingID uuid.UUID           `json:"listing_id"`
	Quantity  int                 `json:"quantity"`
	AddedAt   time.Time           `json:"added_at"`
	Listing   *MarketplaceListing `json:"listing,omitempty"`
}

type InventoryReservation struct {
	ID          uuid.UUID  `json:"id"`
	InventoryID uuid.UUID  `json:"inventory_id"`
	OrderID     *uuid.UUID `json:"order_id"`
	OrgID       uuid.UUID  `json:"org_id"`
	Quantity    int        `json:"quantity"`
	Status      string     `json:"status"`
	ReservedAt  time.Time  `json:"reserved_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	ReleasedAt  *time.Time `json:"released_at"`
}
