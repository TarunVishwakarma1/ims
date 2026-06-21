package testdb

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MustOpen connects to the test database. Skips the test if DATABASE_URL_TEST
// (or DATABASE_URL as a fallback) is unset. Tests must clean up data they
// create (use random keys or t.Cleanup).
func MustOpen(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL_TEST")
	if url == "" {
		url = os.Getenv("DATABASE_URL")
	}
	if url == "" {
		t.Skip("no DATABASE_URL_TEST or DATABASE_URL set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// SeedProduct creates a temporary product for the test, registers cleanup,
// and returns the product id. It auto-picks (or creates) an organization and
// category so tests can run on a fresh database with no seed data.
func SeedProduct(t *testing.T, pool *pgxpool.Pool, name string, pricePaise int64) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	// Pick any existing org, or create a throwaway test org if none exists.
	var orgID uuid.UUID
	err := pool.QueryRow(ctx, `SELECT id FROM organizations LIMIT 1`).Scan(&orgID)
	if err != nil {
		slug := fmt.Sprintf("test-org-%d", time.Now().UnixNano())
		if err2 := pool.QueryRow(ctx, `
			INSERT INTO organizations (name, slug) VALUES ($1, $2) RETURNING id
		`, "Test Org", slug).Scan(&orgID); err2 != nil {
			t.Fatalf("seed: create org: %v", err2)
		}
		// Only register cleanup for orgs that SeedProduct created
		t.Cleanup(func() {
			_, _ = pool.Exec(ctx, `DELETE FROM organizations WHERE id=$1`, orgID)
		})
	}

	// Pick any category from that org, or create a throwaway one.
	var categoryID uuid.UUID
	err = pool.QueryRow(ctx, `SELECT id FROM categories WHERE org_id=$1 LIMIT 1`, orgID).Scan(&categoryID)
	if err != nil {
		if err2 := pool.QueryRow(ctx, `
			INSERT INTO categories (org_id, name) VALUES ($1, $2) RETURNING id
		`, orgID, "test_"+name).Scan(&categoryID); err2 != nil {
			t.Fatalf("seed: pick/create category: %v", err2)
		}
		// Only register cleanup for categories that SeedProduct created
		t.Cleanup(func() {
			_, _ = pool.Exec(ctx, `DELETE FROM categories WHERE id=$1`, categoryID)
		})
	}

	var prodID uuid.UUID
	sku := fmt.Sprintf("TEST-%d", time.Now().UnixNano())
	if err := pool.QueryRow(ctx, `
		INSERT INTO products (org_id, category_id, name, sku, price)
		VALUES ($1, $2, $3, $4, $5) RETURNING id
	`, orgID, categoryID, name, sku, pricePaise).Scan(&prodID); err != nil {
		t.Fatalf("seed product: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM products WHERE id=$1`, prodID)
	})
	return prodID
}

// SeedProductWithStock seeds a product (auto-creating an org and category if
// needed, just like SeedProduct) AND seeds an inventory row with the given
// quantity. Registers cleanup for both. Returns product id and the org id
// it used.
func SeedProductWithStock(t *testing.T, pool *pgxpool.Pool, name string, pricePaise int64, stockQty int) (uuid.UUID, uuid.UUID) {
	t.Helper()
	prodID := SeedProduct(t, pool, name, pricePaise)
	ctx := context.Background()

	var orgID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT org_id FROM products WHERE id=$1`, prodID).Scan(&orgID); err != nil {
		t.Fatalf("seed-stock: lookup org: %v", err)
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO inventory (org_id, product_id, quantity, low_stock_threshold)
		VALUES ($1, $2, $3, 0)
		ON CONFLICT (product_id) DO UPDATE SET quantity = EXCLUDED.quantity
	`, orgID, prodID, stockQty)
	if err != nil {
		t.Fatalf("seed-stock: insert inventory: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM inventory WHERE product_id=$1`, prodID)
	})
	return prodID, orgID
}

// MainOrgIDFromSeed picks the org that owns the most recently seeded product,
// useful when a test needs the org id for service construction.
// In V1 the main shop org is "any existing org" — for tests, you can use
// SeedProductWithStock's returned orgID directly.
func MainOrgIDFromSeed(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var orgID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM organizations LIMIT 1`).Scan(&orgID); err != nil {
		t.Fatalf("MainOrgIDFromSeed: no organization found: %v", err)
	}
	return orgID
}

// StockOf returns the current inventory quantity for a product. Fails the test
// if no inventory row exists.
func StockOf(t *testing.T, pool *pgxpool.Pool, productID uuid.UUID) int {
	t.Helper()
	var qty int
	if err := pool.QueryRow(context.Background(),
		`SELECT quantity FROM inventory WHERE product_id = $1`, productID,
	).Scan(&qty); err != nil {
		t.Fatalf("StockOf: %v", err)
	}
	return qty
}

// SetStock sets the inventory quantity for a product directly. Useful to
// simulate out-of-stock conditions between AddOrSet and Place.
func SetStock(t *testing.T, pool *pgxpool.Pool, productID uuid.UUID, qty int) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`UPDATE inventory SET quantity = $1 WHERE product_id = $2`, qty, productID,
	); err != nil {
		t.Fatalf("SetStock: %v", err)
	}
}

// SeedAddress creates a customer_address row and registers cleanup. Returns the address ID.
func SeedAddress(t *testing.T, pool *pgxpool.Pool, customerID uuid.UUID) uuid.UUID {
	t.Helper()
	addrRepo := repository.NewCustomerAddressRepository(pool)
	id, err := addrRepo.Create(context.Background(), &domain.CustomerAddress{
		CustomerID: customerID,
		Label:      "Home",
		Line1:      "123 Test Street",
		City:       "Mumbai",
		State:      "MH",
		PostalCode: "400001",
	})
	if err != nil {
		t.Fatalf("SeedAddress: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM customer_addresses WHERE id = $1`, id)
	})
	return id
}

// OrderRepo returns a new OrderRepository backed by pool, for use in tests.
func OrderRepo(pool *pgxpool.Pool) repository.OrderRepository {
	return repository.NewOrderRepository(pool)
}

// PickOrFakeOrgID returns any existing org id, or creates a throwaway one.
func PickOrFakeOrgID(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM organizations LIMIT 1`).Scan(&id); err == nil {
		return id
	}
	slug := fmt.Sprintf("test-org-%d", time.Now().UnixNano())
	if err := pool.QueryRow(ctx, `INSERT INTO organizations (name, slug) VALUES ('TestOrg', $1) RETURNING id`, slug).Scan(&id); err != nil {
		t.Fatalf("create org: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM organizations WHERE id=$1`, id) })
	return id
}

// MarkProductShopVisible flips an existing product to shop-visible with the
// supplied slug/description/images. Falls back to nil shopPrice (price column wins).
func MarkProductShopVisible(t *testing.T, pool *pgxpool.Pool, productID uuid.UUID, slug, description string, imageURLs []string, shopPricePaise *int64) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
		UPDATE products
		   SET shop_visible=TRUE, shop_slug=$2, shop_description=$3,
		       shop_image_urls=$4, shop_price_paise=$5
		 WHERE id=$1
	`, productID, slug, description, imageURLs, shopPricePaise); err != nil {
		t.Fatalf("mark visible: %v", err)
	}
}

// SeedOrderForProduct inserts one confirmed b2c orders row + one order_items row
// dated NOW() (within the 30-day popularity window). Registers cleanup for both rows.
// Returns the new order ID.
func SeedOrderForProduct(t *testing.T, pool *pgxpool.Pool, orgID, productID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var orderID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO orders (org_id, status, total_amount, order_type, created_at)
		VALUES ($1, 'confirmed', 100, 'b2c', NOW()) RETURNING id
	`, orgID).Scan(&orderID); err != nil {
		t.Fatalf("seed order: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO order_items (order_id, product_id, quantity, unit_price, org_id)
		VALUES ($1, $2, 1, 100, $3)
	`, orderID, productID, orgID); err != nil {
		t.Fatalf("seed order_item: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM order_items WHERE order_id=$1`, orderID)
		_, _ = pool.Exec(ctx, `DELETE FROM orders WHERE id=$1`, orderID)
	})
	return orderID
}

// SeedShopCategory inserts a category and registers cleanup. Pass shopVisible=true to expose to shop.
func SeedShopCategory(t *testing.T, pool *pgxpool.Pool, orgID uuid.UUID, name, slug string, sortOrder int, shopVisible bool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO categories (org_id, name, slug, sort_order, shop_visible)
		VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		orgID, name, slug, sortOrder, shopVisible,
	).Scan(&id); err != nil {
		t.Fatalf("seed category: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM categories WHERE id=$1`, id) })
	return id
}
