package shop_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/internal/testdb"
)

type fakeRefunder struct {
	called    bool
	gotAmount int64
	gotErr    error
}

func (f *fakeRefunder) Refund(_ context.Context, _, _ uuid.UUID, amount int64, _ string) error {
	f.called = true
	f.gotAmount = amount
	return f.gotErr
}

func seedShopOrder(t *testing.T, pool repository.DBTX, orgID, customerID uuid.UUID, status, paymentStatus string, total int64) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	addr, _ := json.Marshal(map[string]any{"line1": "Test"})
	var id uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO orders (org_id, user_id, customer_id, status, total_amount, order_type,
		                    payment_status, subtotal, delivery_fee, discount,
		                    delivery_address_snapshot)
		VALUES ($1, NULL, $2, $3, $4, 'b2c', $5, $4, 0, 0, $6)
		RETURNING id`,
		orgID, customerID, status, total, paymentStatus, addr,
	).Scan(&id); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return id
}

func TestShopOrder_List_NewestFirst(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.PickOrFakeOrgID(t, pool)
	customerID := testdb.SeedCustomer(t, pool)
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM orders WHERE customer_id=$1`, customerID)
	})

	first := seedShopOrder(t, pool, orgID, customerID, "pending", "unpaid", 100)
	time.Sleep(10 * time.Millisecond)
	second := seedShopOrder(t, pool, orgID, customerID, "pending", "unpaid", 200)

	repo := repository.NewOrderRepository(pool)
	svc := shop.NewShopOrderService(pool, repo, &fakeRefunder{}, orgID)

	res, err := svc.List(context.Background(), customerID, shop.OrderListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 2 {
		t.Fatalf("expected 2, got %d", len(res.Items))
	}
	if res.Items[0].ID != second {
		t.Fatalf("expected newest first")
	}
	if res.Items[1].ID != first {
		t.Fatalf("expected first last")
	}
}

func TestShopOrder_Get_WrongCustomer_NotFound(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.PickOrFakeOrgID(t, pool)
	customerA := testdb.SeedCustomer(t, pool)
	customerB := testdb.SeedCustomer(t, pool)
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM orders WHERE customer_id IN ($1,$2)`, customerA, customerB)
	})

	orderA := seedShopOrder(t, pool, orgID, customerA, "pending", "unpaid", 100)

	repo := repository.NewOrderRepository(pool)
	svc := shop.NewShopOrderService(pool, repo, &fakeRefunder{}, orgID)

	if _, err := svc.Get(context.Background(), customerB, orderA); err == nil {
		t.Fatal("expected error")
	}
}

func TestShopOrder_Get_Cancellable(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.PickOrFakeOrgID(t, pool)
	customerID := testdb.SeedCustomer(t, pool)
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM orders WHERE customer_id=$1`, customerID)
	})

	pendingID := seedShopOrder(t, pool, orgID, customerID, "pending", "unpaid", 100)
	processingID := seedShopOrder(t, pool, orgID, customerID, "processing", "paid", 100)

	repo := repository.NewOrderRepository(pool)
	svc := shop.NewShopOrderService(pool, repo, &fakeRefunder{}, orgID)

	d, _ := svc.Get(context.Background(), customerID, pendingID)
	if !d.Cancellable {
		t.Fatal("pending should be cancellable")
	}
	d, _ = svc.Get(context.Background(), customerID, processingID)
	if d.Cancellable {
		t.Fatal("processing should NOT be cancellable")
	}
}

func TestShopOrder_Cancel_COD_HappyPath(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.PickOrFakeOrgID(t, pool)
	customerID := testdb.SeedCustomer(t, pool)
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM orders WHERE customer_id=$1`, customerID)
	})

	prodID, _ := testdb.SeedProductWithStock(t, pool, "Cancel Test", 100, 10)
	orderID := seedShopOrder(t, pool, orgID, customerID, "pending", "unpaid", 200)
	pool.Exec(context.Background(), `
		INSERT INTO order_items (org_id, order_id, product_id, quantity, unit_price)
		VALUES ($1, $2, $3, 2, 100)`,
		orgID, orderID, prodID,
	)
	pool.Exec(context.Background(), `UPDATE inventory SET quantity = quantity - 2 WHERE product_id=$1`, prodID)

	repo := repository.NewOrderRepository(pool)
	refunder := &fakeRefunder{}
	svc := shop.NewShopOrderService(pool, repo, refunder, orgID)

	res, err := svc.Cancel(context.Background(), customerID, orderID)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if res.Status != "cancelled" {
		t.Fatalf("expected cancelled, got %s", res.Status)
	}
	if res.RefundQueued {
		t.Fatal("COD must not queue refund")
	}
	if refunder.called {
		t.Fatal("Refund must not be called for COD")
	}

	var qty int
	pool.QueryRow(context.Background(), `SELECT quantity FROM inventory WHERE product_id=$1`, prodID).Scan(&qty)
	if qty != 10 {
		t.Fatalf("expected stock restored to 10, got %d", qty)
	}
}

func TestShopOrder_Cancel_WrongStatus_NotAllowed(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.PickOrFakeOrgID(t, pool)
	customerID := testdb.SeedCustomer(t, pool)
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM orders WHERE customer_id=$1`, customerID)
	})

	orderID := seedShopOrder(t, pool, orgID, customerID, "processing", "unpaid", 200)

	repo := repository.NewOrderRepository(pool)
	svc := shop.NewShopOrderService(pool, repo, &fakeRefunder{}, orgID)

	if _, err := svc.Cancel(context.Background(), customerID, orderID); err == nil {
		t.Fatal("expected ErrCancelNotAllowed")
	}
}

func TestShopOrder_Cancel_Razorpay_QueuesRefund(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.PickOrFakeOrgID(t, pool)
	customerID := testdb.SeedCustomer(t, pool)
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM orders WHERE customer_id=$1`, customerID)
	})

	prodID, _ := testdb.SeedProductWithStock(t, pool, "RZP Cancel Test", 100, 10)
	paymentID := uuid.New()
	addr, _ := json.Marshal(map[string]any{"line1": "Test"})
	var orderID uuid.UUID
	pool.QueryRow(context.Background(), `
		INSERT INTO orders (org_id, user_id, customer_id, status, total_amount, order_type,
		                    payment_status, payment_id, subtotal, delivery_fee, discount,
		                    delivery_address_snapshot)
		VALUES ($1, NULL, $2, 'pending', 500, 'b2c', 'paid', $3, 500, 0, 0, $4)
		RETURNING id`,
		orgID, customerID, paymentID, addr,
	).Scan(&orderID)
	pool.Exec(context.Background(), `
		INSERT INTO order_items (org_id, order_id, product_id, quantity, unit_price)
		VALUES ($1, $2, $3, 1, 500)`,
		orgID, orderID, prodID,
	)
	pool.Exec(context.Background(), `UPDATE inventory SET quantity = quantity - 1 WHERE product_id=$1`, prodID)

	repo := repository.NewOrderRepository(pool)
	refunder := &fakeRefunder{}
	svc := shop.NewShopOrderService(pool, repo, refunder, orgID)

	res, err := svc.Cancel(context.Background(), customerID, orderID)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if res.Status != "cancelling" {
		t.Fatalf("expected cancelling, got %s", res.Status)
	}
	if !res.RefundQueued {
		t.Fatal("Razorpay must queue refund")
	}

	// Wait briefly for goroutine.
	for i := 0; i < 50; i++ {
		if refunder.called {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !refunder.called {
		t.Fatal("Refund goroutine did not fire")
	}
	if refunder.gotAmount != 500 {
		t.Fatalf("wrong refund amount: %d", refunder.gotAmount)
	}

	var status string
	pool.QueryRow(context.Background(), `SELECT status FROM orders WHERE id=$1`, orderID).Scan(&status)
	if status != "cancelling" {
		t.Fatalf("expected DB status cancelling, got %s", status)
	}
}
