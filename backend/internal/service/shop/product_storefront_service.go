package shop

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
)

var (
	ErrShopSlugRequired = errors.New("a storefront slug is required to make a product visible")
	ErrShopSlugTaken    = errors.New("that product storefront slug is already used in this shop")
)

// ProductStorefront is a product's consumer-storefront overlay: whether it shows
// in the shop, and its shop-specific slug/price/description/images.
type ProductStorefront struct {
	ProductID       uuid.UUID `json:"product_id"`
	ShopVisible     bool      `json:"shop_visible"`
	ShopSlug        string    `json:"shop_slug"`
	ShopPricePaise  *int64    `json:"shop_price_paise"` // nil → base product price
	ShopDescription string    `json:"shop_description"`
	ShopImageURLs   []string  `json:"shop_image_urls"`
}

// ProductStorefrontService reads and writes a product's storefront overlay,
// scoped to the caller's org.
type ProductStorefrontService interface {
	Get(ctx context.Context, orgID, productID uuid.UUID) (*ProductStorefront, error)
	Set(ctx context.Context, orgID uuid.UUID, in ProductStorefront) (*ProductStorefront, error)
}

type productStorefrontService struct{ pool *pgxpool.Pool }

func NewProductStorefrontService(pool *pgxpool.Pool) ProductStorefrontService {
	return &productStorefrontService{pool: pool}
}

func (s *productStorefrontService) Get(ctx context.Context, orgID, productID uuid.UUID) (*ProductStorefront, error) {
	out := &ProductStorefront{ProductID: productID, ShopImageURLs: []string{}}
	err := s.pool.QueryRow(ctx, `
		SELECT shop_visible, COALESCE(shop_slug,''), shop_price_paise,
		       COALESCE(shop_description,''), COALESCE(shop_image_urls, '{}')
		  FROM products WHERE id=$1 AND org_id=$2`, productID, orgID,
	).Scan(&out.ShopVisible, &out.ShopSlug, &out.ShopPricePaise, &out.ShopDescription, &out.ShopImageURLs)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *productStorefrontService) Set(ctx context.Context, orgID uuid.UUID, in ProductStorefront) (*ProductStorefront, error) {
	slug := strings.ToLower(strings.TrimSpace(in.ShopSlug))
	if in.ShopVisible {
		if slug == "" {
			return nil, ErrShopSlugRequired
		}
		if !slugRe.MatchString(slug) {
			return nil, ErrInvalidProfileSlug
		}
	}
	if in.ShopPricePaise != nil && *in.ShopPricePaise < 0 {
		return nil, errors.New("shop price must not be negative")
	}
	if in.ShopImageURLs == nil {
		in.ShopImageURLs = []string{}
	}

	tag, err := s.pool.Exec(ctx, `
		UPDATE products
		   SET shop_visible=$3, shop_slug=NULLIF($4,''), shop_price_paise=$5,
		       shop_description=$6, shop_image_urls=$7
		 WHERE id=$1 AND org_id=$2`,
		in.ProductID, orgID, in.ShopVisible, slug, in.ShopPricePaise,
		in.ShopDescription, in.ShopImageURLs)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrShopSlugTaken
		}
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, domain.ErrNotFound
	}
	return s.Get(ctx, orgID, in.ProductID)
}
