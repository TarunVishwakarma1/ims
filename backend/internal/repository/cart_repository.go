package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
)

// CartRepository defines persistence operations for a customer's cart.
type CartRepository interface {
	Get(ctx context.Context, customerID uuid.UUID) (*domain.Cart, error)
	UpsertItem(ctx context.Context, customerID, productID uuid.UUID, qty int, unitPricePaise int64) error
	RemoveItem(ctx context.Context, customerID, productID uuid.UUID) error
	Clear(ctx context.Context, customerID uuid.UUID) error
	EnsureCart(ctx context.Context, customerID uuid.UUID) error
	// SetShop binds the cart to a shop org (Zomato-style single-shop cart).
	// Passing uuid.Nil clears the binding (used when the cart empties).
	SetShop(ctx context.Context, customerID, orgID uuid.UUID) error
}

type cartRepository struct{ pool *pgxpool.Pool }

func NewCartRepository(pool *pgxpool.Pool) CartRepository {
	return &cartRepository{pool: pool}
}

// EnsureCart creates a customer_carts row if one doesn't already exist.
// It is idempotent — safe to call multiple times.
func (r *cartRepository) EnsureCart(ctx context.Context, customerID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO customer_carts (customer_id) VALUES ($1) ON CONFLICT DO NOTHING`,
		customerID,
	)
	return err
}

// UpsertItem ensures the cart exists and then inserts or REPLACES the item's
// qty and unit_price_paise. It does NOT accumulate qty.
func (r *cartRepository) UpsertItem(ctx context.Context, customerID, productID uuid.UUID, qty int, unitPricePaise int64) error {
	if err := r.EnsureCart(ctx, customerID); err != nil {
		return err
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO customer_cart_items (customer_id, product_id, qty, unit_price_paise)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (customer_id, product_id)
		DO UPDATE SET qty = EXCLUDED.qty, unit_price_paise = EXCLUDED.unit_price_paise
	`, customerID, productID, qty, unitPricePaise)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx,
		`UPDATE customer_carts SET updated_at = NOW() WHERE customer_id = $1`,
		customerID,
	)
	return err
}

// SetShop binds (or, with uuid.Nil, unbinds) the cart's shop org. Ensures the
// cart row exists first so a binding can be set before the first item lands.
func (r *cartRepository) SetShop(ctx context.Context, customerID, orgID uuid.UUID) error {
	if err := r.EnsureCart(ctx, customerID); err != nil {
		return err
	}
	var org *uuid.UUID
	if orgID != uuid.Nil {
		org = &orgID
	}
	_, err := r.pool.Exec(ctx,
		`UPDATE customer_carts SET org_id = $2, updated_at = NOW() WHERE customer_id = $1`,
		customerID, org,
	)
	return err
}

// RemoveItem deletes a single item from the cart.
func (r *cartRepository) RemoveItem(ctx context.Context, customerID, productID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM customer_cart_items WHERE customer_id = $1 AND product_id = $2`,
		customerID, productID,
	)
	return err
}

// Clear removes all items from the customer's cart and unbinds its shop.
func (r *cartRepository) Clear(ctx context.Context, customerID uuid.UUID) error {
	if _, err := r.pool.Exec(ctx,
		`DELETE FROM customer_cart_items WHERE customer_id = $1`,
		customerID,
	); err != nil {
		return err
	}
	_, err := r.pool.Exec(ctx,
		`UPDATE customer_carts SET org_id = NULL, updated_at = NOW() WHERE customer_id = $1`,
		customerID,
	)
	return err
}

// Get returns the customer's cart with all items ordered by added_at.
// Items is always a non-nil slice (empty if no items).
func (r *cartRepository) Get(ctx context.Context, customerID uuid.UUID) (*domain.Cart, error) {
	cart := &domain.Cart{
		CustomerID: customerID,
		Items:      []domain.CartItem{},
	}
	// Scan updated_at + bound shop if the cart row exists; ignore ErrNoRows
	// (cart not yet created). org_id is NULL for an empty/unbound cart.
	var org *uuid.UUID
	if err := r.pool.QueryRow(ctx,
		`SELECT updated_at, org_id FROM customer_carts WHERE customer_id = $1`,
		customerID,
	).Scan(&cart.UpdatedAt, &org); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if org != nil {
		cart.ShopOrgID = *org
	}

	rows, err := r.pool.Query(ctx, `
		SELECT product_id, qty, unit_price_paise, added_at
		FROM customer_cart_items
		WHERE customer_id = $1
		ORDER BY added_at
	`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var it domain.CartItem
		if err := rows.Scan(&it.ProductID, &it.Qty, &it.UnitPricePaise, &it.AddedAt); err != nil {
			return nil, err
		}
		cart.Items = append(cart.Items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return cart, nil
}
