package shop_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/internal/testdb"
	"github.com/google/uuid"
)

func checkoutRandPhone() string {
	return fmt.Sprintf("+916%09d", time.Now().UnixNano()%1_000_000_000)
}

// TestCheckoutSvc_PlaceCOD_Success seeds a product (stock=5, price=5000),
// adds qty=2 to cart, then places a COD order. Verifies:
//   - res.OrderID is non-zero
//   - res.InvoiceNumber is non-empty
//   - res.PayablePaise == 10000 (qty*price, gst_rate defaults to 0)
//   - stock decremented 5 → 3
//   - orders row has status='confirmed', order_type='b2c', payment_status='unpaid'
//   - order_items has the right row
//   - cart is empty after place
func TestCheckoutSvc_PlaceCOD_Success(t *testing.T) {
	pool := testdb.MustOpen(t)
	ctx := context.Background()

	prodID, orgID := testdb.SeedProductWithStock(t, pool, "CODItem", 5000, 5)

	custRepo := repository.NewCustomerRepository(pool)
	phone := checkoutRandPhone()
	cust, err := custRepo.UpsertByPhone(ctx, phone)
	if err != nil {
		t.Fatalf("upsert customer: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM customers WHERE phone=$1`, phone)
	})

	addrID := testdb.SeedAddress(t, pool, cust.ID)

	cartRepo := repository.NewCartRepository(pool)
	cartSvc := shop.NewCartService(cartRepo, pool, orgID)
	if _, err := cartSvc.AddOrSet(ctx, cust.ID, prodID, 2); err != nil {
		t.Fatalf("AddOrSet: %v", err)
	}

	svc := shop.NewCheckoutService(pool, orgID, cartRepo, repository.NewCustomerAddressRepository(pool), nil, testdb.OrderRepo(pool), "")

	res, err := svc.Place(ctx, shop.PlaceOrderInput{
		CustomerID:    cust.ID,
		AddressID:     addrID,
		PaymentMethod: "cod",
	})
	if err != nil {
		t.Fatalf("Place: %v", err)
	}

	if res.OrderID == (uuid.UUID{}) {
		t.Fatal("expected non-zero OrderID")
	}
	if res.InvoiceNumber == "" {
		t.Fatal("expected non-empty InvoiceNumber")
	}
	if res.PayablePaise != 10000 {
		t.Fatalf("expected PayablePaise=10000, got %d", res.PayablePaise)
	}

	// stock should be 5-2=3
	if got := testdb.StockOf(t, pool, prodID); got != 3 {
		t.Fatalf("expected stock=3, got %d", got)
	}

	// verify orders row
	var status, orderType, paymentStatus string
	if err := pool.QueryRow(ctx,
		`SELECT status, order_type, payment_status FROM orders WHERE id = $1`,
		res.OrderID,
	).Scan(&status, &orderType, &paymentStatus); err != nil {
		t.Fatalf("query order: %v", err)
	}
	if status != "confirmed" {
		t.Errorf("expected status='confirmed', got %q", status)
	}
	if orderType != "b2c" {
		t.Errorf("expected order_type='b2c', got %q", orderType)
	}
	if paymentStatus != "unpaid" {
		t.Errorf("expected payment_status='unpaid', got %q", paymentStatus)
	}

	// verify order_items
	var itemCount int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM order_items WHERE order_id = $1`, res.OrderID,
	).Scan(&itemCount); err != nil {
		t.Fatalf("count order_items: %v", err)
	}
	if itemCount != 1 {
		t.Fatalf("expected 1 order item, got %d", itemCount)
	}

	// cart should be empty
	cart, err := cartRepo.Get(ctx, cust.ID)
	if err != nil {
		t.Fatalf("Get cart: %v", err)
	}
	if len(cart.Items) != 0 {
		t.Fatalf("expected empty cart after place, got %d items", len(cart.Items))
	}
}

// TestCheckoutSvc_PlaceInsufficientStock seeds a product with stock=1, adds
// qty=1 to cart, then sets stock to 0 out-of-band. Place should return
// ErrInsufficientStock and NOT create an order.
func TestCheckoutSvc_PlaceInsufficientStock(t *testing.T) {
	pool := testdb.MustOpen(t)
	ctx := context.Background()

	prodID, orgID := testdb.SeedProductWithStock(t, pool, "RareItem", 5000, 1)

	custRepo := repository.NewCustomerRepository(pool)
	phone := checkoutRandPhone()
	cust, err := custRepo.UpsertByPhone(ctx, phone)
	if err != nil {
		t.Fatalf("upsert customer: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM customers WHERE phone=$1`, phone)
	})

	addrID := testdb.SeedAddress(t, pool, cust.ID)

	cartRepo := repository.NewCartRepository(pool)
	cartSvc := shop.NewCartService(cartRepo, pool, orgID)
	if _, err := cartSvc.AddOrSet(ctx, cust.ID, prodID, 1); err != nil {
		t.Fatalf("AddOrSet: %v", err)
	}

	// drop stock to 0 out-of-band to simulate race
	testdb.SetStock(t, pool, prodID, 0)

	svc := shop.NewCheckoutService(pool, orgID, cartRepo, repository.NewCustomerAddressRepository(pool), nil, testdb.OrderRepo(pool), "")
	_, err = svc.Place(ctx, shop.PlaceOrderInput{
		CustomerID:    cust.ID,
		AddressID:     addrID,
		PaymentMethod: "cod",
	})
	if err != shop.ErrInsufficientStock {
		t.Fatalf("expected ErrInsufficientStock, got %v", err)
	}

	// no order should be created
	var count int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM orders WHERE customer_id = $1`, cust.ID,
	).Scan(&count); err != nil {
		t.Fatalf("count orders: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 orders after insufficient stock, got %d", count)
	}
}

// TestCheckoutSvc_PlaceEmptyCart verifies that Place returns ErrCartEmpty when
// the customer's cart has no items.
func TestCheckoutSvc_PlaceEmptyCart(t *testing.T) {
	pool := testdb.MustOpen(t)
	ctx := context.Background()

	_, orgID := testdb.SeedProductWithStock(t, pool, "EmptyCartProd", 100, 10)

	custRepo := repository.NewCustomerRepository(pool)
	phone := checkoutRandPhone()
	cust, err := custRepo.UpsertByPhone(ctx, phone)
	if err != nil {
		t.Fatalf("upsert customer: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM customers WHERE phone=$1`, phone)
	})

	addrID := testdb.SeedAddress(t, pool, cust.ID)

	cartRepo := repository.NewCartRepository(pool)
	svc := shop.NewCheckoutService(pool, orgID, cartRepo, repository.NewCustomerAddressRepository(pool), nil, testdb.OrderRepo(pool), "")

	_, err = svc.Place(ctx, shop.PlaceOrderInput{
		CustomerID:    cust.ID,
		AddressID:     addrID,
		PaymentMethod: "cod",
	})
	if err != shop.ErrCartEmpty {
		t.Fatalf("expected ErrCartEmpty, got %v", err)
	}
}

// TestCheckoutSvc_PlaceMissingAddress verifies that Place returns ErrAddressRequired
// when AddressID is uuid.Nil.
func TestCheckoutSvc_PlaceMissingAddress(t *testing.T) {
	pool := testdb.MustOpen(t)
	ctx := context.Background()

	_, orgID := testdb.SeedProductWithStock(t, pool, "AddrRequiredProd", 100, 10)

	custRepo := repository.NewCustomerRepository(pool)
	phone := checkoutRandPhone()
	cust, err := custRepo.UpsertByPhone(ctx, phone)
	if err != nil {
		t.Fatalf("upsert customer: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM customers WHERE phone=$1`, phone)
	})

	cartRepo := repository.NewCartRepository(pool)
	svc := shop.NewCheckoutService(pool, orgID, cartRepo, repository.NewCustomerAddressRepository(pool), nil, testdb.OrderRepo(pool), "")

	_, err = svc.Place(ctx, shop.PlaceOrderInput{
		CustomerID:    cust.ID,
		AddressID:     uuid.Nil,
		PaymentMethod: "cod",
	})
	if err != shop.ErrAddressRequired {
		t.Fatalf("expected ErrAddressRequired, got %v", err)
	}
}

// TestCheckoutSvc_Place_AddressNotOwned verifies that Place returns ErrAddressRequired
// when the supplied address belongs to a different customer (IDOR prevention).
func TestCheckoutSvc_Place_AddressNotOwned(t *testing.T) {
	pool := testdb.MustOpen(t)
	ctx := context.Background()

	_, orgID := testdb.SeedProductWithStock(t, pool, "IDORProd", 100, 10)

	custRepo := repository.NewCustomerRepository(pool)

	// Seed customer A with their own address.
	phoneA := checkoutRandPhone()
	custA, err := custRepo.UpsertByPhone(ctx, phoneA)
	if err != nil {
		t.Fatalf("upsert customer A: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM customers WHERE phone=$1`, phoneA)
	})
	addrA := testdb.SeedAddress(t, pool, custA.ID)

	// Seed customer B (no address).
	phoneB := checkoutRandPhone()
	custB, err := custRepo.UpsertByPhone(ctx, phoneB)
	if err != nil {
		t.Fatalf("upsert customer B: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM customers WHERE phone=$1`, phoneB)
	})

	cartRepo := repository.NewCartRepository(pool)
	svc := shop.NewCheckoutService(pool, orgID, cartRepo, repository.NewCustomerAddressRepository(pool), nil, testdb.OrderRepo(pool), "")

	// Customer B attempts to place an order with customer A's address.
	_, err = svc.Place(ctx, shop.PlaceOrderInput{
		CustomerID:    custB.ID,
		AddressID:     addrA,
		PaymentMethod: "cod",
	})
	if err != shop.ErrAddressRequired {
		t.Fatalf("expected ErrAddressRequired (IDOR), got %v", err)
	}
}
