package shop_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/internal/testdb"
)

func cartRandPhone() string {
	return fmt.Sprintf("+917%09d", time.Now().UnixNano()%1_000_000_000)
}

// TestCartSvc_AddClampsToStock seeds a product with stock=3, attempts AddOrSet
// with qty=10, and expects the cart qty to be clamped to 3 with a warning.
func TestCartSvc_AddClampsToStock(t *testing.T) {
	pool := testdb.MustOpen(t)
	ctx := context.Background()

	prodID, orgID := testdb.SeedProductWithStock(t, pool, "WidgetClamp", 9999, 3)

	custRepo := repository.NewCustomerRepository(pool)
	phone := cartRandPhone()
	cust, err := custRepo.UpsertByPhone(ctx, phone)
	if err != nil {
		t.Fatalf("upsert customer: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM customers WHERE phone=$1`, phone)
	})

	svc := shop.NewCartService(repository.NewCartRepository(pool), pool, orgID)

	v, err := svc.AddOrSet(ctx, cust.ID, prodID, 10)
	if err != nil {
		t.Fatalf("AddOrSet: %v", err)
	}
	if len(v.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(v.Items))
	}
	if v.Items[0].Qty != 3 {
		t.Fatalf("expected qty clamped to 3, got %d", v.Items[0].Qty)
	}
	if len(v.Warnings) == 0 {
		t.Fatal("expected stock_clamped warning, got none")
	}
	found := false
	for _, w := range v.Warnings {
		if strings.Contains(w, "stock_clamped") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'stock_clamped' in warnings, got %v", v.Warnings)
	}
}

// TestCartSvc_RemoveAndClear seeds a product, adds it to cart, then removes it;
// expects empty cart.
func TestCartSvc_RemoveAndClear(t *testing.T) {
	pool := testdb.MustOpen(t)
	ctx := context.Background()

	prodID, orgID := testdb.SeedProductWithStock(t, pool, "GadgetRemove", 1000, 50)

	custRepo := repository.NewCustomerRepository(pool)
	phone := cartRandPhone()
	cust, err := custRepo.UpsertByPhone(ctx, phone)
	if err != nil {
		t.Fatalf("upsert customer: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM customers WHERE phone=$1`, phone)
	})

	svc := shop.NewCartService(repository.NewCartRepository(pool), pool, orgID)

	if _, err := svc.AddOrSet(ctx, cust.ID, prodID, 2); err != nil {
		t.Fatalf("AddOrSet: %v", err)
	}

	v, err := svc.Remove(ctx, cust.ID, prodID)
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(v.Items) != 0 {
		t.Fatalf("expected empty cart after remove, got %d items", len(v.Items))
	}
}

// TestCartSvc_GetCleansMissingProduct adds a product to the cart, deletes the
// product directly from DB, then calls Get — expects empty Items and the
// product id in RemovedItems.
func TestCartSvc_GetCleansMissingProduct(t *testing.T) {
	pool := testdb.MustOpen(t)
	ctx := context.Background()

	prodID, orgID := testdb.SeedProductWithStock(t, pool, "EphemeralProduct", 500, 10)

	custRepo := repository.NewCustomerRepository(pool)
	phone := cartRandPhone()
	cust, err := custRepo.UpsertByPhone(ctx, phone)
	if err != nil {
		t.Fatalf("upsert customer: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM customers WHERE phone=$1`, phone)
	})

	svc := shop.NewCartService(repository.NewCartRepository(pool), pool, orgID)

	if _, err := svc.AddOrSet(ctx, cust.ID, prodID, 1); err != nil {
		t.Fatalf("AddOrSet: %v", err)
	}

	// Move the product to a different real org so loadSnap returns pgx.ErrNoRows
	// for our orgID, while the cart item FK (products.id) stays intact.
	// This simulates a product being transferred out of the shop org.
	var altOrgID uuid.UUID
	altSlug := fmt.Sprintf("alt-org-%d", time.Now().UnixNano())
	if err := pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ($1, $2) RETURNING id`,
		"Alt Org", altSlug,
	).Scan(&altOrgID); err != nil {
		t.Fatalf("create alt org: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM organizations WHERE id=$1`, altOrgID)
	})
	if _, err := pool.Exec(ctx, `UPDATE products SET org_id=$1 WHERE id=$2`, altOrgID, prodID); err != nil {
		t.Fatalf("move product to alt org: %v", err)
	}
	t.Cleanup(func() {
		// Restore org so the SeedProduct cleanup can delete it
		_, _ = pool.Exec(ctx, `UPDATE products SET org_id=$1 WHERE id=$2`, orgID, prodID)
	})

	v, err := svc.Get(ctx, cust.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(v.Items) != 0 {
		t.Fatalf("expected 0 items after product deletion, got %d", len(v.Items))
	}
	if len(v.RemovedItems) == 0 {
		t.Fatal("expected RemovedItems to contain deleted product id")
	}
	found := false
	for _, rid := range v.RemovedItems {
		if rid == prodID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected prodID %v in RemovedItems %v", prodID, v.RemovedItems)
	}
}

// TestCartSvc_AddOrSetReplacesQty verifies that AddOrSet replaces rather than
// accumulates qty (upsert REPLACE semantics).
func TestCartSvc_AddOrSetReplacesQty(t *testing.T) {
	pool := testdb.MustOpen(t)
	ctx := context.Background()

	prodID, orgID := testdb.SeedProductWithStock(t, pool, "ReplaceWidget", 200, 100)

	custRepo := repository.NewCustomerRepository(pool)
	phone := cartRandPhone()
	cust, err := custRepo.UpsertByPhone(ctx, phone)
	if err != nil {
		t.Fatalf("upsert customer: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM customers WHERE phone=$1`, phone)
	})

	svc := shop.NewCartService(repository.NewCartRepository(pool), pool, orgID)

	if _, err := svc.AddOrSet(ctx, cust.ID, prodID, 2); err != nil {
		t.Fatalf("AddOrSet qty=2: %v", err)
	}

	v, err := svc.AddOrSet(ctx, cust.ID, prodID, 5)
	if err != nil {
		t.Fatalf("AddOrSet qty=5: %v", err)
	}
	if len(v.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(v.Items))
	}
	if v.Items[0].Qty != 5 {
		t.Fatalf("expected qty=5 (replace semantics), got %d", v.Items[0].Qty)
	}
}

// TestCartSvc_RejectZeroQty verifies that AddOrSet(qty=0) returns an error
// containing "qty must be positive".
func TestCartSvc_RejectZeroQty(t *testing.T) {
	pool := testdb.MustOpen(t)
	ctx := context.Background()

	prodID, orgID := testdb.SeedProductWithStock(t, pool, "ZeroQtyProduct", 100, 50)

	custRepo := repository.NewCustomerRepository(pool)
	phone := cartRandPhone()
	cust, err := custRepo.UpsertByPhone(ctx, phone)
	if err != nil {
		t.Fatalf("upsert customer: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM customers WHERE phone=$1`, phone)
	})

	svc := shop.NewCartService(repository.NewCartRepository(pool), pool, orgID)

	_, err = svc.AddOrSet(ctx, cust.ID, prodID, 0)
	if err == nil {
		t.Fatal("expected error for qty=0, got nil")
	}
	if !strings.Contains(err.Error(), "qty must be positive") {
		t.Fatalf("expected 'qty must be positive' in error, got: %v", err)
	}
}
