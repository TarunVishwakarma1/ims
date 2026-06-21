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
