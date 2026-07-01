package shop

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrShopNotFound is returned when a slug doesn't map to a live shop.
var ErrShopNotFound = errors.New("shop not found")

// ShopSummary is a directory entry — one live consumer shop.
type ShopSummary struct {
	Slug     string   `json:"slug"`
	Name     string   `json:"name"`
	Tagline  string   `json:"tagline,omitempty"`
	LogoURL  string   `json:"logo_url,omitempty"`
	Area     string   `json:"area,omitempty"`
	City     string   `json:"city,omitempty"`
	Pincodes []string `json:"pincodes"`
	// DistanceKm is set only for location-based (ListNearby) results.
	DistanceKm *float64 `json:"distance_km,omitempty"`
}

// ShopDirectoryService lists live consumer shops, optionally filtered to those
// that deliver to a given pincode.
type ShopDirectoryService interface {
	List(ctx context.Context, pincode string) ([]ShopSummary, error)
	// ListNearby returns live shops whose delivery radius covers the given
	// point, nearest first. Shops without lat/lng or delivery_radius_km are
	// excluded (they're pincode-serviceable only).
	ListNearby(ctx context.Context, lat, lng float64) ([]ShopSummary, error)
	// OrgBySlug resolves a live shop's slug to its owning org id.
	OrgBySlug(ctx context.Context, slug string) (uuid.UUID, error)
}

type shopDirectoryService struct {
	pool *pgxpool.Pool
}

func NewShopDirectoryService(pool *pgxpool.Pool) ShopDirectoryService {
	return &shopDirectoryService{pool: pool}
}

func (s *shopDirectoryService) List(ctx context.Context, pincode string) ([]ShopSummary, error) {
	q := `
		SELECT slug, display_name, tagline, logo_url, area, city, pincodes
		  FROM shop_profiles
		 WHERE is_live = TRUE`
	args := []any{}
	if pincode != "" {
		args = append(args, pincode)
		q += ` AND $1 = ANY(pincodes)`
	}
	q += ` ORDER BY display_name`

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []ShopSummary{}
	for rows.Next() {
		var sm ShopSummary
		if err := rows.Scan(&sm.Slug, &sm.Name, &sm.Tagline, &sm.LogoURL, &sm.Area, &sm.City, &sm.Pincodes); err != nil {
			return nil, err
		}
		out = append(out, sm)
	}
	return out, rows.Err()
}

func (s *shopDirectoryService) ListNearby(ctx context.Context, lat, lng float64) ([]ShopSummary, error) {
	// Great-circle distance via the haversine/spherical-law-of-cosines form.
	// LEAST(1, …) guards acos against floating-point values slightly above 1
	// (which happen when the point coincides with a shop). The distance alias
	// can't be used in WHERE, so filter in an outer query.
	const q = `
		SELECT slug, display_name, tagline, logo_url, area, city, pincodes, distance_km
		  FROM (
			SELECT slug, display_name, tagline, logo_url, area, city, pincodes,
			       delivery_radius_km,
			       6371 * acos(LEAST(1,
			           cos(radians($1)) * cos(radians(lat)) * cos(radians(lng) - radians($2))
			         + sin(radians($1)) * sin(radians(lat))
			       )) AS distance_km
			  FROM shop_profiles
			 WHERE is_live = TRUE
			   AND lat IS NOT NULL AND lng IS NOT NULL
			   AND delivery_radius_km IS NOT NULL
		  ) t
		 WHERE distance_km <= delivery_radius_km
		 ORDER BY distance_km`

	rows, err := s.pool.Query(ctx, q, lat, lng)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []ShopSummary{}
	for rows.Next() {
		var sm ShopSummary
		var dist float64
		if err := rows.Scan(&sm.Slug, &sm.Name, &sm.Tagline, &sm.LogoURL, &sm.Area, &sm.City, &sm.Pincodes, &dist); err != nil {
			return nil, err
		}
		d := dist
		sm.DistanceKm = &d
		out = append(out, sm)
	}
	return out, rows.Err()
}

func (s *shopDirectoryService) OrgBySlug(ctx context.Context, slug string) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx,
		`SELECT org_id FROM shop_profiles WHERE slug = $1 AND is_live = TRUE`, slug,
	).Scan(&id)
	if err != nil {
		return uuid.Nil, ErrShopNotFound
	}
	return id, nil
}
