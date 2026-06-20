package shop

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TarunVishwakarma1/ims/backend/pkg/cache"
)

var ErrNotFound = errors.New("not found")

type CategoryView struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	Slug    string    `json:"slug"`
	IconURL string    `json:"icon_url,omitempty"`
}

type ProductCard struct {
	ID           uuid.UUID `json:"id"`
	Slug         string    `json:"slug"`
	Name         string    `json:"name"`
	PricePaise   int64     `json:"price_paise"`
	ImageURL     string    `json:"image_url,omitempty"`
	AvailableQty int       `json:"available_qty"`
	CategorySlug string    `json:"category_slug,omitempty"`
}

type ProductDetail struct {
	ProductCard
	Description  string   `json:"description"`
	ImageURLs    []string `json:"image_urls"`
	GSTRate      int      `json:"gst_rate"`
	CategoryName string   `json:"category_name,omitempty"`
}

type ProductListQuery struct {
	CategorySlug  string
	Search        string
	PriceMinPaise *int64
	PriceMaxPaise *int64
	InStockOnly   bool
	Sort          string
	Limit         int
	Offset        int
	Cursor        string
}

type ProductListResult struct {
	Items      []ProductCard `json:"items"`
	TotalCount int           `json:"total_count"`
	Limit      int           `json:"limit"`
	Offset     int           `json:"offset,omitempty"`
	NextCursor string        `json:"next_cursor,omitempty"`
}

type CatalogService interface {
	ListCategories(ctx context.Context) ([]CategoryView, error)
	ListProducts(ctx context.Context, q ProductListQuery) (*ProductListResult, error)
	GetProductBySlug(ctx context.Context, slug string) (*ProductDetail, error)
	InvalidateCategories(ctx context.Context) error
	InvalidateProductList(ctx context.Context) error
	InvalidateProduct(ctx context.Context, slug string) error
}

type catalogService struct {
	pool  *pgxpool.Pool
	cache cache.Cache
	orgID uuid.UUID
}

func NewCatalogService(pool *pgxpool.Pool, c cache.Cache, orgID uuid.UUID) CatalogService {
	return &catalogService{pool, c, orgID}
}

func (s *catalogService) ListCategories(ctx context.Context) ([]CategoryView, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, COALESCE(slug,''), COALESCE(icon_url,'')
		  FROM categories
		 WHERE org_id=$1 AND shop_visible=TRUE AND slug IS NOT NULL
		 ORDER BY sort_order, name
	`, s.orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CategoryView{}
	for rows.Next() {
		var v CategoryView
		if err := rows.Scan(&v.ID, &v.Name, &v.Slug, &v.IconURL); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Stubs — fleshed out in subsequent tasks. They MUST compile (so the
// interface is satisfied) but return ErrNotFound or empty results.

func (s *catalogService) ListProducts(ctx context.Context, q ProductListQuery) (*ProductListResult, error) {
	return &ProductListResult{Items: []ProductCard{}}, nil
}
func (s *catalogService) GetProductBySlug(ctx context.Context, slug string) (*ProductDetail, error) {
	return nil, ErrNotFound
}
func (s *catalogService) InvalidateCategories(ctx context.Context) error              { return nil }
func (s *catalogService) InvalidateProductList(ctx context.Context) error             { return nil }
func (s *catalogService) InvalidateProduct(ctx context.Context, slug string) error    { return nil }
