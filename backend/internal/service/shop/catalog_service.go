package shop

import (
	"context"
	"errors"
	"fmt"
	"strings"

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

func (s *catalogService) ListProducts(ctx context.Context, q ProductListQuery) (*ProductListResult, error) {
	q = normalizeQuery(q)
	if err := validateSort(q.Sort); err != nil {
		return nil, err
	}
	if q.PriceMinPaise != nil && q.PriceMaxPaise != nil && *q.PriceMinPaise > *q.PriceMaxPaise {
		return nil, errors.New("invalid price range")
	}

	whereSQL, args := s.buildWhere(q)
	orderSQL := s.buildOrderBy(q)

	listSQL := `
		SELECT p.id, COALESCE(p.shop_slug,''), p.name,
		       COALESCE(p.shop_price_paise, p.price),
		       COALESCE(p.shop_image_urls[1], ''),
		       COALESCE(i.quantity, 0),
		       COALESCE(c.slug, '')
		  FROM products p
		  LEFT JOIN inventory i ON i.product_id = p.id
		  LEFT JOIN categories c ON c.id = p.category_id
		 ` + whereSQL + ` ` + orderSQL + `
		 LIMIT ` + fmt.Sprintf("$%d", len(args)+1) + ` OFFSET ` + fmt.Sprintf("$%d", len(args)+2)
	args = append(args, q.Limit, q.Offset)

	rows, err := s.pool.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProductCard{}
	for rows.Next() {
		var p ProductCard
		if err := rows.Scan(&p.ID, &p.Slug, &p.Name, &p.PricePaise, &p.ImageURL, &p.AvailableQty, &p.CategorySlug); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Count (without LIMIT/OFFSET).
	countArgs := args[:len(args)-2]
	countSQL := `SELECT COUNT(*) FROM products p LEFT JOIN inventory i ON i.product_id=p.id LEFT JOIN categories c ON c.id=p.category_id ` + whereSQL
	var total int
	if err := s.pool.QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, err
	}

	return &ProductListResult{Items: out, TotalCount: total, Limit: q.Limit, Offset: q.Offset}, nil
}

func normalizeQuery(q ProductListQuery) ProductListQuery {
	if q.Limit <= 0 {
		q.Limit = 24
	}
	if q.Limit > 100 {
		q.Limit = 100
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
	if q.Sort == "" {
		q.Sort = "newest"
	}
	return q
}

func validateSort(s string) error {
	switch s {
	case "newest", "price_asc", "price_desc", "popular", "relevance":
		return nil
	}
	return errors.New("invalid_sort")
}

func (s *catalogService) buildWhere(q ProductListQuery) (string, []any) {
	args := []any{s.orgID}
	clauses := []string{`p.org_id = $1`, `p.shop_visible = TRUE`}
	if q.CategorySlug != "" {
		args = append(args, q.CategorySlug)
		clauses = append(clauses, fmt.Sprintf(`c.slug = $%d`, len(args)))
	}
	if q.PriceMinPaise != nil {
		args = append(args, *q.PriceMinPaise)
		clauses = append(clauses, fmt.Sprintf(`COALESCE(p.shop_price_paise,p.price) >= $%d`, len(args)))
	}
	if q.PriceMaxPaise != nil {
		args = append(args, *q.PriceMaxPaise)
		clauses = append(clauses, fmt.Sprintf(`COALESCE(p.shop_price_paise,p.price) <= $%d`, len(args)))
	}
	if q.InStockOnly {
		clauses = append(clauses, `COALESCE(i.quantity,0) > 0`)
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func (s *catalogService) buildOrderBy(q ProductListQuery) string {
	switch q.Sort {
	case "price_asc":
		return `ORDER BY COALESCE(p.shop_price_paise,p.price) ASC, p.id ASC`
	case "price_desc":
		return `ORDER BY COALESCE(p.shop_price_paise,p.price) DESC, p.id ASC`
	case "popular":
		return `ORDER BY (
		SELECT COUNT(*) FROM order_items oi
		  JOIN orders o ON o.id = oi.order_id
		 WHERE oi.product_id = p.id AND o.created_at > NOW() - INTERVAL '30 days'
	) DESC, p.created_at DESC, p.id ASC`
	}
	return `ORDER BY p.created_at DESC, p.id ASC` // newest
}
func (s *catalogService) GetProductBySlug(ctx context.Context, slug string) (*ProductDetail, error) {
	return nil, ErrNotFound
}
func (s *catalogService) InvalidateCategories(ctx context.Context) error              { return nil }
func (s *catalogService) InvalidateProductList(ctx context.Context) error             { return nil }
func (s *catalogService) InvalidateProduct(ctx context.Context, slug string) error    { return nil }
