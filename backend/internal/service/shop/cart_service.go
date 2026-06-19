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
	AddOrSet(ctx context.Context, customerID, productID uuid.UUID, qty int) (*CartView, error)
	Remove(ctx context.Context, customerID, productID uuid.UUID) (*CartView, error)
	Clear(ctx context.Context, customerID uuid.UUID) error
	Merge(ctx context.Context, customerID uuid.UUID, items []MergeItem) (*CartView, error)
}

// CartView is the read model returned to callers.
type CartView struct {
	Items         []CartItemView `json:"items"`
	SubtotalPaise int64          `json:"subtotal_paise"`
	RemovedItems  []uuid.UUID    `json:"removed_items,omitempty"`
	Warnings      []string       `json:"warnings,omitempty"`
}

// CartItemView represents a single line item in the cart view.
type CartItemView struct {
	ProductID      uuid.UUID `json:"product_id"`
	Name           string    `json:"name"`
	ImageURL       string    `json:"image_url,omitempty"`
	Qty            int       `json:"qty"`
	UnitPricePaise int64     `json:"unit_price_paise"`
	AvailableQty   int       `json:"available_qty"`
}

// MergeItem is a single entry in a Merge request.
type MergeItem struct {
	ProductID uuid.UUID
	Qty       int
}

// productSnap holds a lightweight snapshot of a product + its current stock.
// V1 simplified: no shopVisible, no imageURL (Plan 2 columns).
type productSnap struct {
	name      string
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

// loadSnap queries the product name, price, and current inventory for the
// given product within the service's org. Returns pgx.ErrNoRows if the product
// does not exist (or is not in the shop org).
//
// V1: every product in the shop org is visible. shop_visible column lands in Plan 2 (000017).
func (s *cartService) loadSnap(ctx context.Context, productID uuid.UUID) (*productSnap, error) {
	sp := &productSnap{}
	err := s.pool.QueryRow(ctx, `
		SELECT p.name,
		       p.price AS unit_price_paise,
		       COALESCE(i.quantity, 0) AS available
		  FROM products p
		  LEFT JOIN inventory i ON i.product_id = p.id
		 WHERE p.id = $1 AND p.org_id = $2
	`, productID, s.orgID).Scan(&sp.name, &sp.priceP, &sp.available)
	if err != nil {
		return nil, err
	}
	return sp, nil
}

// AddOrSet sets (replaces) the qty of a product in the customer's cart.
// qty must be positive. qty is clamped to available stock; if clamped-to-zero,
// returns "out of stock". A "stock_clamped" warning is appended to the returned
// CartView when clamping occurs.
func (s *cartService) AddOrSet(ctx context.Context, customerID, productID uuid.UUID, qty int) (*CartView, error) {
	if qty <= 0 {
		return nil, errors.New("qty must be positive")
	}

	sp, err := s.loadSnap(ctx, productID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("product not found")
		}
		return nil, err
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

	for _, it := range cart.Items {
		sp, err := s.loadSnap(ctx, it.ProductID)
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
			Name:           sp.name,
			ImageURL:       "", // V1: no image_url column yet (Plan 2)
			Qty:            qty,
			UnitPricePaise: sp.priceP,
			AvailableQty:   sp.available,
		})
		v.SubtotalPaise += int64(qty) * sp.priceP
	}

	return v, nil
}

// Remove deletes a single item from the customer's cart and returns the
// updated CartView.
func (s *cartService) Remove(ctx context.Context, customerID, productID uuid.UUID) (*CartView, error) {
	if err := s.repo.RemoveItem(ctx, customerID, productID); err != nil {
		return nil, err
	}
	return s.Get(ctx, customerID)
}

// Clear removes all items from the customer's cart.
func (s *cartService) Clear(ctx context.Context, customerID uuid.UUID) error {
	return s.repo.Clear(ctx, customerID)
}

// Merge applies a list of MergeItem entries to the customer's cart via AddOrSet.
// Errors for individual items (e.g. product not found, out of stock) are
// swallowed so a partial guest-cart merge does not fail entirely.
func (s *cartService) Merge(ctx context.Context, customerID uuid.UUID, items []MergeItem) (*CartView, error) {
	for _, it := range items {
		if _, err := s.AddOrSet(ctx, customerID, it.ProductID, it.Qty); err != nil {
			// Swallow per-item errors (missing product, out of stock, etc.)
			continue
		}
	}
	return s.Get(ctx, customerID)
}
