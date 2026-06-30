package shop

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
)

// V1: every product in the shop org is visible. shop_visible column lands in Plan 2 (000017).

// CartService manages a customer's shopping cart with stock clamping.
type CartService interface {
	Get(ctx context.Context, customerID uuid.UUID) (*CartView, error)
	AddOrSet(ctx context.Context, customerID, productID uuid.UUID, qty int, replace bool) (*CartView, error)
	Remove(ctx context.Context, customerID, productID uuid.UUID) (*CartView, error)
	Clear(ctx context.Context, customerID uuid.UUID) error
	Merge(ctx context.Context, customerID uuid.UUID, items []MergeItem) (*CartView, error)
}

// CartShopConflict is returned by AddOrSet when the cart already holds items
// from a different shop and replace was not requested. It carries the current
// shop's identity so the UI can prompt "start a new cart?".
type CartShopConflict struct {
	CurrentSlug string
	CurrentName string
}

func (e *CartShopConflict) Error() string { return "cart_other_shop" }

// CartShop identifies the single shop a cart is bound to.
type CartShop struct {
	OrgID uuid.UUID `json:"org_id"`
	Slug  string    `json:"slug"`
	Name  string    `json:"name"`
}

// CartView is the read model returned to callers.
type CartView struct {
	Items         []CartItemView `json:"items"`
	SubtotalPaise int64          `json:"subtotal_paise"`
	Shop          *CartShop      `json:"shop,omitempty"`
	RemovedItems  []uuid.UUID    `json:"removed_items,omitempty"`
	Warnings      []string       `json:"warnings,omitempty"`
}

// CartItemView represents a single line item in the cart view.
// Field names align with frontend CartItem (slug/image/max_qty).
type CartItemView struct {
	ProductID      uuid.UUID `json:"product_id"`
	Slug           string    `json:"slug"`
	Name           string    `json:"name"`
	Image          string    `json:"image"`
	Qty            int       `json:"qty"`
	UnitPricePaise int64     `json:"unit_price_paise"`
	MaxQty         int       `json:"max_qty"`
}

// MergeItem is a single entry in a Merge request.
type MergeItem struct {
	ProductID uuid.UUID
	Qty       int
}

// productSnap holds a lightweight snapshot of a product + its current stock.
type productSnap struct {
	slug      string
	name      string
	imageURL  string
	priceP    int64
	available int
}

type cartService struct {
	repo  repository.CartRepository
	pool  *pgxpool.Pool
	orgID uuid.UUID
}

// NewCartService constructs a CartService that uses the provided CartRepository,
// a raw pgxpool for product/inventory queries, and orgID as the single-tenant
// shop organisation identifier (V1).
func NewCartService(r repository.CartRepository, pool *pgxpool.Pool, mainOrgID uuid.UUID) CartService {
	return &cartService{repo: r, pool: pool, orgID: mainOrgID}
}

// org returns the per-request shop org (slug-resolved by ResolveShop) or the
// default org. AddOrSet/Merge run inside a shop storefront, so the request
// carries the shop; Get/Remove run on the global cart and derive the shop from
// the stored cart instead.
func (s *cartService) org(ctx context.Context) uuid.UUID {
	if id, ok := shopOrgFromContext(ctx); ok {
		return id
	}
	return s.orgID
}

// shopInfo resolves a shop org's slug + display name for the cart view. Returns
// empty strings when the org has no live profile (shouldn't happen for a bound
// cart, but kept non-fatal).
func (s *cartService) shopInfo(ctx context.Context, orgID uuid.UUID) (slug, name string) {
	_ = s.pool.QueryRow(ctx,
		`SELECT slug, display_name FROM shop_profiles WHERE org_id = $1`,
		orgID,
	).Scan(&slug, &name)
	return slug, name
}

// loadSnap queries the product name, price, and current inventory for the
// given product within the given shop org. Returns pgx.ErrNoRows if the product
// does not exist (or is not in that shop).
//
// V1: every product in the shop org is visible. shop_visible column lands in Plan 2 (000017).
func (s *cartService) loadSnap(ctx context.Context, orgID, productID uuid.UUID) (*productSnap, error) {
	sp := &productSnap{}
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(p.shop_slug, ''),
		       p.name,
		       COALESCE(p.shop_image_urls[1], '') AS image_url,
		       COALESCE(p.shop_price_paise, p.price) AS unit_price_paise,
		       COALESCE(i.quantity, 0) AS available
		  FROM products p
		  LEFT JOIN inventory i ON i.product_id = p.id
		 WHERE p.id = $1 AND p.org_id = $2
	`, productID, orgID).Scan(&sp.slug, &sp.name, &sp.imageURL, &sp.priceP, &sp.available)
	if err != nil {
		return nil, err
	}
	return sp, nil
}

// AddOrSet sets (replaces) the qty of a product in the customer's cart.
// qty must be positive. qty is clamped to available stock; if clamped-to-zero,
// returns "out of stock". A "stock_clamped" warning is appended to the returned
// CartView when clamping occurs.
//
// The cart is bound to a single shop (Zomato-style). The shop is taken from the
// request (ResolveShop). If the cart already holds items from a different shop
// and replace is false, returns *CartShopConflict so the UI can prompt; with
// replace true the existing cart is cleared first and rebound to this shop.
func (s *cartService) AddOrSet(ctx context.Context, customerID, productID uuid.UUID, qty int, replace bool) (*CartView, error) {
	if qty <= 0 {
		return nil, errors.New("qty must be positive")
	}

	shopOrg := s.org(ctx)

	sp, err := s.loadSnap(ctx, shopOrg, productID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("product not found")
		}
		return nil, err
	}

	// Single-shop guard: a non-empty cart bound to a different shop blocks the
	// add unless the caller explicitly replaces it.
	cart, err := s.repo.Get(ctx, customerID)
	if err != nil {
		return nil, err
	}
	if len(cart.Items) > 0 && cart.ShopOrgID != uuid.Nil && cart.ShopOrgID != shopOrg {
		if !replace {
			slug, name := s.shopInfo(ctx, cart.ShopOrgID)
			return nil, &CartShopConflict{CurrentSlug: slug, CurrentName: name}
		}
		if err := s.repo.Clear(ctx, customerID); err != nil {
			return nil, err
		}
	}

	warning := ""
	if qty > sp.available {
		qty = sp.available
		warning = "stock_clamped"
	}
	if qty == 0 {
		return nil, errors.New("out of stock")
	}

	if err := s.repo.UpsertItem(ctx, customerID, productID, qty, sp.priceP); err != nil {
		return nil, err
	}
	// Bind (or rebind) the cart to this shop now that it holds an item.
	if err := s.repo.SetShop(ctx, customerID, shopOrg); err != nil {
		return nil, err
	}

	v, err := s.Get(ctx, customerID)
	if err != nil {
		return nil, err
	}
	if warning != "" {
		v.Warnings = append(v.Warnings, warning)
	}
	return v, nil
}

// Get rebuilds the CartView for the customer, validating each item against
// current product/stock data. Products that no longer exist are silently
// removed from the cart and listed in RemovedItems. Qty is clamped in the view
// (but NOT persisted) when available stock is less than the stored qty.
func (s *cartService) Get(ctx context.Context, customerID uuid.UUID) (*CartView, error) {
	cart, err := s.repo.Get(ctx, customerID)
	if err != nil {
		return nil, err
	}

	v := &CartView{Items: []CartItemView{}}

	// Items belong to the cart's bound shop — snap against THAT org, not the
	// request org (Get runs on the global cart route with no shop in context).
	snapOrg := cart.ShopOrgID
	if snapOrg == uuid.Nil {
		snapOrg = s.org(ctx)
	}

	for _, it := range cart.Items {
		sp, err := s.loadSnap(ctx, snapOrg, it.ProductID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// Product no longer exists — remove from cart, report to caller.
				v.RemovedItems = append(v.RemovedItems, it.ProductID)
				_ = s.repo.RemoveItem(ctx, customerID, it.ProductID)
				continue
			}
			return nil, err
		}

		qty := it.Qty
		if qty > sp.available {
			// Runtime clamp — do NOT persist; just warn the caller.
			qty = sp.available
			v.Warnings = append(v.Warnings, "stock_clamped")
		}

		v.Items = append(v.Items, CartItemView{
			ProductID:      it.ProductID,
			Slug:           sp.slug,
			Name:           sp.name,
			Image:          sp.imageURL,
			Qty:            qty,
			UnitPricePaise: sp.priceP,
			MaxQty:         sp.available,
		})
		v.SubtotalPaise += int64(qty) * sp.priceP
	}

	if cart.ShopOrgID != uuid.Nil && len(v.Items) > 0 {
		slug, name := s.shopInfo(ctx, cart.ShopOrgID)
		v.Shop = &CartShop{OrgID: cart.ShopOrgID, Slug: slug, Name: name}
	}

	return v, nil
}

// Remove deletes a single item from the customer's cart and returns the
// updated CartView. When the cart empties, its shop binding is cleared so the
// next add can pick any shop.
func (s *cartService) Remove(ctx context.Context, customerID, productID uuid.UUID) (*CartView, error) {
	if err := s.repo.RemoveItem(ctx, customerID, productID); err != nil {
		return nil, err
	}
	v, err := s.Get(ctx, customerID)
	if err != nil {
		return nil, err
	}
	if len(v.Items) == 0 {
		_ = s.repo.SetShop(ctx, customerID, uuid.Nil)
	}
	return v, nil
}

// Clear removes all items from the customer's cart.
func (s *cartService) Clear(ctx context.Context, customerID uuid.UUID) error {
	return s.repo.Clear(ctx, customerID)
}

// Merge folds a guest cart (single-shop) into the customer's server cart on
// login. The guest cart belongs to the shop in context; replace=true so an
// existing server cart from a different shop is superseded by the just-chosen
// guest cart rather than erroring on the first item. Per-item errors (missing
// product, out of stock) are swallowed so a partial merge still succeeds.
func (s *cartService) Merge(ctx context.Context, customerID uuid.UUID, items []MergeItem) (*CartView, error) {
	for i, it := range items {
		// Only the first item may need to clear a stale other-shop cart.
		if _, err := s.AddOrSet(ctx, customerID, it.ProductID, it.Qty, i == 0); err != nil {
			continue
		}
	}
	return s.Get(ctx, customerID)
}
