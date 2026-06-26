package testdb

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// ShopOrder is a minimal view of an orders row used by shop service tests.
type ShopOrder struct {
	ID            uuid.UUID
	Status        string
	PaymentStatus string
}

// SeedB2COrderPendingRazorpay creates:
//   - an org (or reuses one that exists)
//   - a customer row
//   - an orders row (status='pending', payment_status='unpaid', order_type='b2c')
//   - a payments row (status='created') with a synthetic razorpay_order_id
//
// Returns (pool, orgID, orderID, customerID, razorpayOrderID).
// All rows are removed via t.Cleanup in reverse-insert order.
func SeedB2COrderPendingRazorpay(t *testing.T) (*pgxpool.Pool, uuid.UUID, uuid.UUID, uuid.UUID, string) {
	t.Helper()
	pool := MustOpen(t)
	ctx := context.Background()

	// Reuse any existing org or create a throwaway one.
	orgID := PickOrFakeOrgID(t, pool)

	// Create a customer.
	customerID := uuid.New()
	name := fmt.Sprintf("shop-test-cust-%d", time.Now().UnixNano())
	email := fmt.Sprintf("%s@example.com", name)
	_, err := pool.Exec(ctx,
		`INSERT INTO customers (id, name, email, is_guest) VALUES ($1, $2, $3, true)`,
		customerID, name, email,
	)
	require.NoError(t, err)

	// Insert order. user_id is nullable (migration 000015 dropped NOT NULL).
	orderID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO orders (
			id, org_id, customer_id,
			status, order_type, total_amount, subtotal,
			payment_status, created_at, updated_at
		) VALUES ($1, $2, $3, 'pending', 'b2c', 59000, 50000, 'unpaid', NOW(), NOW())
	`, orderID, orgID, customerID)
	require.NoError(t, err)

	// Insert payment row.
	paymentID := uuid.New()
	rzpOrderID := fmt.Sprintf("order_TEST_%d", time.Now().UnixNano())
	_, err = pool.Exec(ctx, `
		INSERT INTO payments (id, org_id, order_id, razorpay_order_id, amount, currency, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 59000, 'INR', 'created', NOW(), NOW())
	`, paymentID, orgID, orderID, rzpOrderID)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM payments WHERE id = $1`, paymentID)
		_, _ = pool.Exec(ctx, `DELETE FROM orders WHERE id = $1`, orderID)
		_, _ = pool.Exec(ctx, `DELETE FROM customers WHERE id = $1`, customerID)
	})

	return pool, orgID, orderID, customerID, rzpOrderID
}

// SeedB2COrderPaid creates a B2C order in the already-paid state
// (status='confirmed', payment_status='paid', payment status='captured').
// Returns (pool, orgID, orderID, customerID, razorpayOrderID).
func SeedB2COrderPaid(t *testing.T) (*pgxpool.Pool, uuid.UUID, uuid.UUID, uuid.UUID, string) {
	t.Helper()
	pool, orgID, orderID, customerID, rzpOrderID := SeedB2COrderPendingRazorpay(t)
	ctx := context.Background()

	_, err := pool.Exec(ctx, `
		UPDATE orders SET status = 'confirmed', payment_status = 'paid', updated_at = NOW() WHERE id = $1
	`, orderID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		UPDATE payments SET status = 'captured', updated_at = NOW() WHERE order_id = $1
	`, orderID)
	require.NoError(t, err)

	return pool, orgID, orderID, customerID, rzpOrderID
}

// GetShopOrder reads a minimal orders row by id. Useful in assertions.
func GetShopOrder(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) *ShopOrder {
	t.Helper()
	var o ShopOrder
	err := pool.QueryRow(context.Background(),
		`SELECT id, status, payment_status FROM orders WHERE id = $1`, id,
	).Scan(&o.ID, &o.Status, &o.PaymentStatus)
	require.NoError(t, err)
	return &o
}
