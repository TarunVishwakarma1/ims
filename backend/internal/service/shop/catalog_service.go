package shop

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/TarunVishwakarma1/ims/backend/pkg/cache"
)

var ErrNotFound = errors.New("not found")

var searchSafe = regexp.MustCompile(`[^a-z0-9\s]+`)

func sanitizeSearch(q string) string {
	q = strings.ToLower(strings.TrimSpace(q))
	q = searchSafe.ReplaceAllString(q, " ")
	if len(q) > 100 {
		q = q[:100]
	}
	return strings.TrimSpace(q)
}

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
	key := fmt.Sprintf(keyCategories, s.orgID)
	var cached []CategoryView
	if err := s.cache.Get(ctx, key, &cached); err == nil {
		return cached, nil
	}
	out, err := s.listCategoriesFromDB(ctx)
	if err != nil {
		return nil, err
	}
	_ = s.cache.Set(ctx, key, out, ttlMedium)
	return out, nil
}

func (s *catalogService) listCategoriesFromDB(ctx context.Context) ([]CategoryView, error) {
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

	key := fmt.Sprintf(keyProductList, s.orgID, plistHash(q))
	var cached ProductListResult
	if err := s.cache.Get(ctx, key, &cached); err == nil {
		return &cached, nil
	}
	out, err := s.listProductsFromDB(ctx, q)
	if err != nil {
		return nil, err
	}
	_ = s.cache.Set(ctx, key, out, ttlShort)
	return out, nil
}

func (s *catalogService) listProductsFromDB(ctx context.Context, q ProductListQuery) (*ProductListResult, error) {
	if term := sanitizeSearch(q.Search); len(term) >= 2 {
		if q.Sort == "" || q.Sort == "newest" {
			q.Sort = "relevance"
		}
		return s.searchProducts(ctx, q, term)
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

	result := &ProductListResult{Items: out, TotalCount: total, Limit: q.Limit, Offset: q.Offset}

	if len(out) == q.Limit && len(out) > 0 {
		last := out[len(out)-1]
		var key any
		switch q.Sort {
		case "price_asc", "price_desc":
			key = last.PricePaise
		default:
			// Re-fetch created_at for the last row — cheap single point lookup.
			_ = s.pool.QueryRow(ctx, `SELECT created_at FROM products WHERE id=$1`, last.ID).Scan(&key)
		}
		result.NextCursor = encodeCursor(key, last.ID)
	}

	return result, nil
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
	if q.Cursor != "" {
		q.Offset = 0
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
	if q.Cursor != "" {
		cp, err := decodeCursor(q.Cursor)
		if err != nil {
			zap.L().Debug("shop catalog: cursor decode failed, falling back to first page", zap.Error(err))
		} else {
			switch q.Sort {
			case "price_asc":
				// ASC, ASC — tuple > works
				args = append(args, cp.SortKey, cp.LastID)
				clauses = append(clauses,
					fmt.Sprintf(`(COALESCE(p.shop_price_paise,p.price), p.id) > ($%d, $%d)`, len(args)-1, len(args)))
			case "price_desc":
				// DESC, ASC — mixed directions: tuple comparison invalid, expand manually
				args = append(args, cp.SortKey, cp.LastID)
				clauses = append(clauses,
					fmt.Sprintf(`(COALESCE(p.shop_price_paise,p.price) < $%d OR (COALESCE(p.shop_price_paise,p.price) = $%d AND p.id > $%d))`, len(args)-1, len(args)-1, len(args)))
			default:
				// newest: created_at DESC, id ASC — mixed directions, expand manually.
				// TODO(Task 9): popular sort needs its own cursor (currently uses created_at).
				args = append(args, cp.SortKey, cp.LastID)
				clauses = append(clauses,
					fmt.Sprintf(`(p.created_at < $%d OR (p.created_at = $%d AND p.id > $%d))`, len(args)-1, len(args)-1, len(args)))
			}
		}
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
func (s *catalogService) searchProducts(ctx context.Context, q ProductListQuery, term string) (*ProductListResult, error) {
	args := []any{s.orgID, term}
	clauses := []string{`p.org_id = $1`, `p.shop_visible = TRUE`,
		`(p.search_vector @@ plainto_tsquery('english', $2) OR word_similarity($2, p.name) > 0.2)`}
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
	where := "WHERE " + strings.Join(clauses, " AND ")
	listSQL := `
		WITH ranked AS (
		  SELECT p.id, COALESCE(p.shop_slug,'') AS slug, p.name,
		         COALESCE(p.shop_price_paise, p.price) AS price_paise,
		         COALESCE(p.shop_image_urls[1], '') AS image_url,
		         COALESCE(i.quantity, 0) AS available_qty,
		         COALESCE(c.slug, '') AS category_slug,
		         ts_rank(p.search_vector, plainto_tsquery('english', $2)) AS fts_rank,
		         word_similarity($2, p.name) AS trgm_sim
		    FROM products p
		    LEFT JOIN inventory i ON i.product_id = p.id
		    LEFT JOIN categories c ON c.id = p.category_id
		    ` + where + `
		)
		SELECT id, slug, name, price_paise, image_url, available_qty, category_slug
		  FROM ranked
		 ORDER BY (fts_rank * 2 + trgm_sim) DESC, name ASC
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

	countArgs := args[:len(args)-2]
	countSQL := `
		SELECT COUNT(*) FROM products p
		  LEFT JOIN inventory i ON i.product_id = p.id
		  LEFT JOIN categories c ON c.id = p.category_id
		  ` + where
	var total int
	if err := s.pool.QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, err
	}

	return &ProductListResult{Items: out, TotalCount: total, Limit: q.Limit, Offset: q.Offset}, nil
}

func (s *catalogService) GetProductBySlug(ctx context.Context, slug string) (*ProductDetail, error) {
	key := fmt.Sprintf(keyProductDetail, s.orgID, slug)
	var cached ProductDetail
	if err := s.cache.Get(ctx, key, &cached); err == nil {
		return &cached, nil
	}
	out, err := s.getProductFromDB(ctx, slug)
	if err != nil {
		return nil, err
	}
	_ = s.cache.Set(ctx, key, out, ttlMedium)
	return out, nil
}

func (s *catalogService) getProductFromDB(ctx context.Context, slug string) (*ProductDetail, error) {
	var d ProductDetail
	err := s.pool.QueryRow(ctx, `
		SELECT p.id, COALESCE(p.shop_slug,''), p.name,
		       COALESCE(p.shop_price_paise, p.price),
		       COALESCE(p.shop_image_urls[1], ''),
		       COALESCE(i.quantity, 0),
		       COALESCE(c.slug, ''),
		       COALESCE(p.shop_description, ''),
		       COALESCE(p.shop_image_urls, '{}'::text[]),
		       COALESCE(p.gst_rate, 0),
		       COALESCE(c.name, '')
		  FROM products p
		  LEFT JOIN inventory i ON i.product_id = p.id
		  LEFT JOIN categories c ON c.id = p.category_id
		 WHERE p.org_id=$1 AND p.shop_slug=$2 AND p.shop_visible=TRUE
	`, s.orgID, slug).Scan(
		&d.ID, &d.Slug, &d.Name, &d.PricePaise, &d.ImageURL, &d.AvailableQty, &d.CategorySlug,
		&d.Description, &d.ImageURLs, &d.GSTRate, &d.CategoryName,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *catalogService) InvalidateCategories(ctx context.Context) error {
	return s.cache.Delete(ctx, fmt.Sprintf(keyCategories, s.orgID))
}
func (s *catalogService) InvalidateProductList(ctx context.Context) error {
	return s.cache.DeleteByPattern(ctx, fmt.Sprintf("shop:plist:%s:*", s.orgID))
}
func (s *catalogService) InvalidateProduct(ctx context.Context, slug string) error {
	return s.cache.Delete(ctx, fmt.Sprintf(keyProductDetail, s.orgID, slug))
}

// Cursor helpers.

type cursorPayload struct {
	SortKey any       `json:"k"`
	LastID  uuid.UUID `json:"id"`
}

func encodeCursor(sortKey any, lastID uuid.UUID) string {
	b, _ := json.Marshal(cursorPayload{SortKey: sortKey, LastID: lastID})
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(s string) (cursorPayload, error) {
	var c cursorPayload
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return c, err
	}
	return c, nil
}
