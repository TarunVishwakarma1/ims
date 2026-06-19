package domain

import (
	"time"

	"github.com/google/uuid"
)

// Partner is a counterparty organization derived from orders — either a
// supplier we've bought from or a buyer who's bought from us. Aggregated
// from the orders table; no separate table exists.
type Partner struct {
	OrgID        uuid.UUID  `json:"org_id"`
	Name         string     `json:"name"`
	Slug         string     `json:"slug"`
	Role         string     `json:"role"` // "supplier" or "buyer"
	OrderCount   int        `json:"order_count"`
	TotalValue   int64      `json:"total_value"`
	PendingCount int        `json:"pending_count"`
	LastOrderAt  *time.Time `json:"last_order_at"`
}

// PartnerDetail bundles partner aggregates with the recent order list and
// the underlying organization record.
type PartnerDetail struct {
	Organization *Organization `json:"organization"`
	AsSupplier   *Partner      `json:"as_supplier,omitempty"` // they sold to us
	AsBuyer      *Partner      `json:"as_buyer,omitempty"`    // they bought from us
	RecentOrders []*Order      `json:"recent_orders"`
}
