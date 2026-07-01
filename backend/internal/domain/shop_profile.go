package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ShopProfile is an org's consumer-facing storefront (1:1 with organizations).
// Only is_live profiles appear in the Kirana customer directory.
type ShopProfile struct {
	OrgID       uuid.UUID `json:"org_id"`
	Slug        string    `json:"slug"`
	DisplayName string    `json:"display_name"`
	Tagline     string    `json:"tagline"`
	LogoURL     string    `json:"logo_url"`
	Area        string    `json:"area"`
	City        string    `json:"city"`
	Pincodes    []string  `json:"pincodes"`
	Lat         *float64  `json:"lat"`
	Lng         *float64  `json:"lng"`
	// DeliveryRadiusKm, when set, makes the shop serviceable by distance from
	// lat/lng. NULL falls back to pincode-only matching.
	DeliveryRadiusKm *float64 `json:"delivery_radius_km"`
	// OpensAt/ClosesAt are "HH:MM" IST business hours; nil either side = always
	// open. ClosesAt < OpensAt wraps past midnight.
	OpensAt          *string   `json:"opens_at"`
	ClosesAt         *string   `json:"closes_at"`
	IsLive           bool      `json:"is_live"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type ShopProfileRepository interface {
	GetByOrg(ctx context.Context, orgID uuid.UUID) (*ShopProfile, error)
	Upsert(ctx context.Context, p *ShopProfile) error
	SlugTakenByOther(ctx context.Context, slug string, orgID uuid.UUID) (bool, error)
}
