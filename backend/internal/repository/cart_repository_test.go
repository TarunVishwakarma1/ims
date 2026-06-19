package repository_test

import (
	"context"
	"testing"

	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/internal/testdb"
)

func TestCartRepo_AddUpsertRemoveClear(t *testing.T) {
	pool := testdb.MustOpen(t)
	custRepo := repository.NewCustomerRepository(pool)
	cartRepo := repository.NewCartRepository(pool)
	ctx := context.Background()

	c, err := custRepo.UpsertByPhone(ctx, "+919999900301")
	if err != nil {
		t.Fatal(err)
	}
	prodID := testdb.SeedProduct(t, pool, "Test prod", 9999)

	if err := cartRepo.EnsureCart(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if err := cartRepo.UpsertItem(ctx, c.ID, prodID, 2, 9999); err != nil {
		t.Fatal(err)
	}
	cart, err := cartRepo.Get(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(cart.Items) != 1 || cart.Items[0].Qty != 2 {
		t.Fatalf("unexpected cart: %+v", cart)
	}

	// Upsert replaces qty (does not sum)
	if err := cartRepo.UpsertItem(ctx, c.ID, prodID, 5, 9999); err != nil {
		t.Fatal(err)
	}
	cart, err = cartRepo.Get(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cart.Items[0].Qty != 5 {
		t.Fatalf("expected qty=5 got %d", cart.Items[0].Qty)
	}

	if err := cartRepo.RemoveItem(ctx, c.ID, prodID); err != nil {
		t.Fatal(err)
	}
	cart, err = cartRepo.Get(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(cart.Items) != 0 {
		t.Fatalf("expected empty cart, got %d items", len(cart.Items))
	}
}

func TestCartRepo_GetEmpty(t *testing.T) {
	pool := testdb.MustOpen(t)
	custRepo := repository.NewCustomerRepository(pool)
	cartRepo := repository.NewCartRepository(pool)
	ctx := context.Background()

	c, err := custRepo.UpsertByPhone(ctx, "+919999900302")
	if err != nil {
		t.Fatal(err)
	}
	// EnsureCart so the row exists, then Get should return empty Items slice
	if err := cartRepo.EnsureCart(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	cart, err := cartRepo.Get(ctx, c.ID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if cart == nil {
		t.Fatal("expected non-nil cart")
	}
	if len(cart.Items) != 0 {
		t.Fatalf("expected empty Items slice, got %d items", len(cart.Items))
	}
}

func TestCartRepo_EnsureCartIdempotent(t *testing.T) {
	pool := testdb.MustOpen(t)
	custRepo := repository.NewCustomerRepository(pool)
	cartRepo := repository.NewCartRepository(pool)
	ctx := context.Background()

	c, err := custRepo.UpsertByPhone(ctx, "+919999900303")
	if err != nil {
		t.Fatal(err)
	}
	// Call EnsureCart twice; both must succeed with no error
	if err := cartRepo.EnsureCart(ctx, c.ID); err != nil {
		t.Fatalf("first EnsureCart: %v", err)
	}
	if err := cartRepo.EnsureCart(ctx, c.ID); err != nil {
		t.Fatalf("second EnsureCart: %v", err)
	}
}
