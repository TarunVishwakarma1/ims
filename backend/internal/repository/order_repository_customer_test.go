package repository_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/internal/testdb"
)

func seedCustomerOrder(t *testing.T, pool repository.DBTX, orgID, customerID uuid.UUID, status string, total int64) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	addr, _ := json.Marshal(map[string]any{"line1": "Test"})
	if err := pool.QueryRow(ctx, `
		INSERT INTO orders (org_id, user_id, customer_id, status, total_amount, order_type,
		                    payment_status, subtotal, delivery_fee, discount,
		                    delivery_address_snapshot)
		VALUES ($1, NULL, $2, $3, $4, 'b2c', 'unpaid', $4, 0, 0, $5)
		RETURNING id`,
		orgID, customerID, status, total, addr,
	).Scan(&id); err != nil {
		t.Fatalf("seed order: %v", err)
	}
	return id
}

func TestOrderRepo_ListByCustomer(t *testing.T) {
	pool := testdb.MustOpen(t)
	repo := repository.NewOrderRepository(pool)

	orgID := testdb.PickOrFakeOrgID(t, pool)
	customerID := testdb.SeedCustomer(t, pool)
	defer pool.Exec(context.Background(), `DELETE FROM orders WHERE customer_id=$1`, customerID)

	ids := make([]uuid.UUID, 3)
	for i := range ids {
		ids[i] = seedCustomerOrder(t, pool, orgID, customerID, "pending", int64(100*(i+1)))
		time.Sleep(10 * time.Millisecond)
	}

	rows, err := repo.ListByCustomer(context.Background(), customerID, time.Time{}, uuid.Nil, 24)
	if err != nil { t.Fatal(err) }
	if len(rows) != 3 { t.Fatalf("expected 3 rows, got %d", len(rows)) }
	// DESC order: newest first
	if rows[0].ID != ids[2] || rows[2].ID != ids[0] {
		t.Fatalf("wrong order: %+v", rows)
	}
}

func TestOrderRepo_GetByCustomerAndID_OwnershipGuard(t *testing.T) {
	pool := testdb.MustOpen(t)
	repo := repository.NewOrderRepository(pool)

	orgID := testdb.PickOrFakeOrgID(t, pool)
	customerA := testdb.SeedCustomer(t, pool)
	customerB := testdb.SeedCustomer(t, pool)
	defer pool.Exec(context.Background(), `DELETE FROM orders WHERE customer_id IN ($1,$2)`, customerA, customerB)

	orderA := seedCustomerOrder(t, pool, orgID, customerA, "pending", 500)

	// Customer A can fetch.
	got, items, err := repo.GetByCustomerAndID(context.Background(), customerA, orderA)
	if err != nil { t.Fatal(err) }
	if got.ID != orderA { t.Fatalf("wrong id: %s", got.ID) }
	_ = items

	// Customer B cannot.
	if _, _, err := repo.GetByCustomerAndID(context.Background(), customerB, orderA); err == nil {
		t.Fatal("expected error for wrong customer")
	}
}
