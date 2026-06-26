package shop_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
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

	svc := shop.NewCheckoutService(pool, orgID, cartRepo, repository.NewCustomerAddressRepository(pool), nil, testdb.OrderRepo(pool), "", 10000, 500000, 300)

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

	svc := shop.NewCheckoutService(pool, orgID, cartRepo, repository.NewCustomerAddressRepository(pool), nil, testdb.OrderRepo(pool), "", 10000, 500000, 300)
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
	svc := shop.NewCheckoutService(pool, orgID, cartRepo, repository.NewCustomerAddressRepository(pool), nil, testdb.OrderRepo(pool), "", 10000, 500000, 300)

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
	svc := shop.NewCheckoutService(pool, orgID, cartRepo, repository.NewCustomerAddressRepository(pool), nil, testdb.OrderRepo(pool), "", 10000, 500000, 300)

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
	svc := shop.NewCheckoutService(pool, orgID, cartRepo, repository.NewCustomerAddressRepository(pool), nil, testdb.OrderRepo(pool), "", 10000, 500000, 300)

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

// TestCheckoutSvc_PaymentOptions_EmptyCart seeds no cart items. COD should be
// enabled (empty cart passes the gate — no order value to check).
func TestCheckoutSvc_PaymentOptions_EmptyCart(t *testing.T) {
	pool := testdb.MustOpen(t)
	ctx := context.Background()

	_, orgID := testdb.SeedProductWithStock(t, pool, "POEmptyProd", 5000, 10)

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
	svc := shop.NewCheckoutService(pool, orgID, cartRepo, repository.NewCustomerAddressRepository(pool), nil, testdb.OrderRepo(pool), "", 10000, 500000, 300)

	opts, err := svc.PaymentOptions(ctx, cust.ID)
	if err != nil {
		t.Fatalf("PaymentOptions: %v", err)
	}
	if len(opts) != 2 {
		t.Fatalf("expected 2 payment options, got %d", len(opts))
	}
	cod := findPaymentOption(opts, "cod")
	if cod == nil {
		t.Fatal("expected 'cod' in payment options")
	}
	if !cod.Enabled {
		t.Errorf("expected cod enabled for empty cart, reason=%q", cod.Reason)
	}
	if cod.Reason != "" {
		t.Errorf("expected empty reason for enabled cod, got %q", cod.Reason)
	}
}

// TestCheckoutSvc_PaymentOptions_UnderMin seeds a cart whose total is below
// codMinPaise. COD should be disabled with reason "min_value_below".
func TestCheckoutSvc_PaymentOptions_UnderMin(t *testing.T) {
	pool := testdb.MustOpen(t)
	ctx := context.Background()

	// Product price 500 paise × qty 1 = 500 total; min is 10000.
	prodID, orgID := testdb.SeedProductWithStock(t, pool, "POUnderMin", 500, 10)

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
	cartSvc := shop.NewCartService(cartRepo, pool, orgID)
	if _, err := cartSvc.AddOrSet(ctx, cust.ID, prodID, 1); err != nil {
		t.Fatalf("AddOrSet: %v", err)
	}

	svc := shop.NewCheckoutService(pool, orgID, cartRepo, repository.NewCustomerAddressRepository(pool), nil, testdb.OrderRepo(pool), "", 10000, 500000, 300)

	opts, err := svc.PaymentOptions(ctx, cust.ID)
	if err != nil {
		t.Fatalf("PaymentOptions: %v", err)
	}
	cod := findPaymentOption(opts, "cod")
	if cod == nil {
		t.Fatal("expected 'cod' in payment options")
	}
	if cod.Enabled {
		t.Error("expected cod disabled when cart is below min")
	}
	if cod.Reason != "min_value_below" {
		t.Errorf("expected reason 'min_value_below', got %q", cod.Reason)
	}
}

// TestCheckoutSvc_PaymentOptions_OverMax seeds a cart whose total exceeds
// codMaxPaise. COD should be disabled with reason "max_value_exceeded".
func TestCheckoutSvc_PaymentOptions_OverMax(t *testing.T) {
	pool := testdb.MustOpen(t)
	ctx := context.Background()

	// Product price 600000 paise × qty 1 = 600000 total; max is 500000.
	prodID, orgID := testdb.SeedProductWithStock(t, pool, "POOverMax", 600000, 5)

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
	cartSvc := shop.NewCartService(cartRepo, pool, orgID)
	if _, err := cartSvc.AddOrSet(ctx, cust.ID, prodID, 1); err != nil {
		t.Fatalf("AddOrSet: %v", err)
	}

	svc := shop.NewCheckoutService(pool, orgID, cartRepo, repository.NewCustomerAddressRepository(pool), nil, testdb.OrderRepo(pool), "", 10000, 500000, 300)

	opts, err := svc.PaymentOptions(ctx, cust.ID)
	if err != nil {
		t.Fatalf("PaymentOptions: %v", err)
	}
	cod := findPaymentOption(opts, "cod")
	if cod == nil {
		t.Fatal("expected 'cod' in payment options")
	}
	if cod.Enabled {
		t.Error("expected cod disabled when cart exceeds max")
	}
	if cod.Reason != "max_value_exceeded" {
		t.Errorf("expected reason 'max_value_exceeded', got %q", cod.Reason)
	}
}

// TestCheckoutSvc_PaymentOptions_InRange seeds a cart whose total is within
// COD bounds. COD should be enabled.
func TestCheckoutSvc_PaymentOptions_InRange(t *testing.T) {
	pool := testdb.MustOpen(t)
	ctx := context.Background()

	// Product price 50000 paise × qty 1 = 50000 total; bounds 10000–500000.
	prodID, orgID := testdb.SeedProductWithStock(t, pool, "POInRange", 50000, 10)

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
	cartSvc := shop.NewCartService(cartRepo, pool, orgID)
	if _, err := cartSvc.AddOrSet(ctx, cust.ID, prodID, 1); err != nil {
		t.Fatalf("AddOrSet: %v", err)
	}

	svc := shop.NewCheckoutService(pool, orgID, cartRepo, repository.NewCustomerAddressRepository(pool), nil, testdb.OrderRepo(pool), "", 10000, 500000, 300)

	opts, err := svc.PaymentOptions(ctx, cust.ID)
	if err != nil {
		t.Fatalf("PaymentOptions: %v", err)
	}
	if len(opts) != 2 {
		t.Fatalf("expected 2 payment options, got %d", len(opts))
	}
	cod := findPaymentOption(opts, "cod")
	if cod == nil {
		t.Fatal("expected 'cod' in payment options")
	}
	if !cod.Enabled {
		t.Errorf("expected cod enabled for in-range cart, reason=%q", cod.Reason)
	}
	if cod.Reason != "" {
		t.Errorf("expected empty reason for enabled cod, got %q", cod.Reason)
	}
	rzp := findPaymentOption(opts, "razorpay")
	if rzp == nil || !rzp.Enabled {
		t.Error("expected razorpay enabled")
	}
}

// TestPlace_COD_UnderMin verifies that Place returns ErrCODIneligible when
// cart total is below codMinPaise and payment method is COD.
func TestPlace_COD_UnderMin_ReturnsIneligible(t *testing.T) {
	pool := testdb.MustOpen(t)
	ctx := context.Background()

	// Product price 5000 paise (₹50), qty 1 = 5000 total; min is 10000.
	prodID, orgID := testdb.SeedProductWithStock(t, pool, "CODUnderMin", 5000, 10)

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

	svc := shop.NewCheckoutService(pool, orgID, cartRepo, repository.NewCustomerAddressRepository(pool), nil, testdb.OrderRepo(pool), "", 10000, 500000, 300)

	_, err = svc.Place(ctx, shop.PlaceOrderInput{
		CustomerID:    cust.ID,
		AddressID:     addrID,
		PaymentMethod: "cod",
	})
	if err != shop.ErrCODIneligible {
		t.Fatalf("expected ErrCODIneligible, got %v", err)
	}
}

// TestPlace_COD_OverMax verifies that Place returns ErrCODIneligible when
// cart total exceeds codMaxPaise and payment method is COD.
func TestPlace_COD_OverMax_ReturnsIneligible(t *testing.T) {
	pool := testdb.MustOpen(t)
	ctx := context.Background()

	// Product price 600000 paise (₹6000), qty 1 = 600000 total; max is 500000.
	prodID, orgID := testdb.SeedProductWithStock(t, pool, "CODOverMax", 600000, 5)

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

	svc := shop.NewCheckoutService(pool, orgID, cartRepo, repository.NewCustomerAddressRepository(pool), nil, testdb.OrderRepo(pool), "", 10000, 500000, 300)

	_, err = svc.Place(ctx, shop.PlaceOrderInput{
		CustomerID:    cust.ID,
		AddressID:     addrID,
		PaymentMethod: "cod",
	})
	if err != shop.ErrCODIneligible {
		t.Fatalf("expected ErrCODIneligible, got %v", err)
	}
}

// TestPlace_COD_InRange_Places verifies that Place succeeds when cart total
// is within COD bounds and payment method is COD.
func TestPlace_COD_InRange_Places(t *testing.T) {
	pool := testdb.MustOpen(t)
	ctx := context.Background()

	// Product price 50000 paise (₹500), qty 1 = 50000 total; bounds 10000–500000.
	prodID, orgID := testdb.SeedProductWithStock(t, pool, "CODInRange", 50000, 10)

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

	svc := shop.NewCheckoutService(pool, orgID, cartRepo, repository.NewCustomerAddressRepository(pool), nil, testdb.OrderRepo(pool), "", 10000, 500000, 300)

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
	if res.PayablePaise != 50000 {
		t.Fatalf("expected PayablePaise=50000, got %d", res.PayablePaise)
	}
}

// TestPlace_Razorpay_NotGatedByCODBounds verifies that Place succeeds when
// payment method is razorpay, even if the cart total exceeds COD max bounds.
// This confirms the COD gate does not apply to non-COD payment methods.
func TestPlace_Razorpay_NotGatedByCODBounds(t *testing.T) {
	pool := testdb.MustOpen(t)
	ctx := context.Background()

	// Product price 600000 paise (₹6000), qty 1 = 600000 total; exceeds max 500000.
	prodID, orgID := testdb.SeedProductWithStock(t, pool, "RZPNotGated", 600000, 5)

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

	// Create a minimal stub paymentCreator that returns a fake payment.
	stubPayment := &MockPaymentCreator{
		OnCreateOrder: func(ctx context.Context, orgID, orderID uuid.UUID, amount int64) (*domain.Payment, error) {
			return &domain.Payment{
				ID:              uuid.New(),
				RazorpayOrderID: "order_test_" + orderID.String()[:8],
			}, nil
		},
	}

	svc := shop.NewCheckoutService(pool, orgID, cartRepo, repository.NewCustomerAddressRepository(pool), stubPayment, testdb.OrderRepo(pool), "test_key_id", 10000, 500000, 300)

	res, err := svc.Place(ctx, shop.PlaceOrderInput{
		CustomerID:    cust.ID,
		AddressID:     addrID,
		PaymentMethod: "razorpay",
	})
	if err != nil {
		t.Fatalf("Place with razorpay: %v", err)
	}

	if res.OrderID == (uuid.UUID{}) {
		t.Fatal("expected non-zero OrderID")
	}
	if res.PayablePaise != 600000 {
		t.Fatalf("expected PayablePaise=600000, got %d", res.PayablePaise)
	}
	if res.RazorpayKeyID != "test_key_id" {
		t.Fatalf("expected RazorpayKeyID='test_key_id', got %q", res.RazorpayKeyID)
	}
}

// findPaymentOption returns the PaymentOption with the given ID or nil.
func findPaymentOption(opts []shop.PaymentOption, id string) *shop.PaymentOption {
	for i := range opts {
		if opts[i].ID == id {
			return &opts[i]
		}
	}
	return nil
}

// MockPaymentCreator is a stub paymentCreator for testing.
type MockPaymentCreator struct {
	OnCreateOrder func(ctx context.Context, orgID, orderID uuid.UUID, amount int64) (*domain.Payment, error)
}

func (m *MockPaymentCreator) CreateOrder(ctx context.Context, orgID, orderID uuid.UUID, amount int64) (*domain.Payment, error) {
	if m.OnCreateOrder != nil {
		return m.OnCreateOrder(ctx, orgID, orderID, amount)
	}
	return nil, fmt.Errorf("not implemented")
}
