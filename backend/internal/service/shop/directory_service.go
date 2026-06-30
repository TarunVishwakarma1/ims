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
}

// ShopDirectoryService lists live consumer shops, optionally filtered to those
// that deliver to a given pincode.
type ShopDirectoryService interface {
	List(ctx context.Context, pincode string) ([]ShopSummary, error)
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
