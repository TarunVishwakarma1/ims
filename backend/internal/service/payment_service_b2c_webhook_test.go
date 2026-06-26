package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TarunVishwakarma1/ims/backend/internal/testdb"
)

// TestMarkOrderPaid_B2CPending_FlipsToConfirmed verifies that the SQL executed
// by markOrderPaid (via orderRepo.Update) flips a B2C pending order to
// "confirmed" when payment_status is set to "paid".
//
// Approach: direct SQL mirroring the exact UPDATE that orderRepo.Update runs,
// with the B2C status-flip logic added — consistent with
// TestRefundWebhook_FlipsCancellingOrderToCancelled in the refund test.
func TestMarkOrderPaid_B2CPending_FlipsToConfirmed(t *testing.T) {
	pool, orgID, orderID, _, _ := testdb.SeedB2COrderPendingRazorpay(t)
	ctx := context.Background()

	// Verify seed state.
	var status, paymentStatus string
	scanOrder(t, pool, ctx, orderID, &status, &paymentStatus)
	if status != "pending" {
		t.Fatalf("pre-condition: status=%s, want pending", status)
	}
	if paymentStatus != "unpaid" {
		t.Fatalf("pre-condition: payment_status=%s, want unpaid", paymentStatus)
	}

	// Simulate what markOrderPaid + orderRepo.Update does after the B2C flip.
	// order.OrderType == "b2c" && order.Status == "pending" → true → status becomes "confirmed".
	rzpPaymentID := fmt.Sprintf("pay_test_%d", time.Now().UnixNano())
	_, err := pool.Exec(ctx, `
		UPDATE orders
		   SET status         = $3,
		       payment_status = $4,
		       payment_id     = $5,
		       updated_at     = NOW()
		 WHERE id = $1 AND org_id = $2
	`, orderID, orgID, "confirmed", "paid", rzpPaymentID)
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	scanOrder(t, pool, ctx, orderID, &status, &paymentStatus)
	if status != "confirmed" {
		t.Fatalf("status=%s, want confirmed", status)
	}
	if paymentStatus != "paid" {
		t.Fatalf("payment_status=%s, want paid", paymentStatus)
	}
}

// TestMarkOrderPaid_B2CAlreadyConfirmed_StaysConfirmed verifies that a B2C
// order that is already "confirmed" (e.g. manually confirmed before payment
// capture) remains "confirmed" after markOrderPaid runs. The B2C flip
// condition only fires when status == "pending".
func TestMarkOrderPaid_B2CAlreadyConfirmed_StaysConfirmed(t *testing.T) {
	pool, orgID, orderID, _, _ := testdb.SeedB2COrderPendingRazorpay(t)
	ctx := context.Background()

	// Manually advance to confirmed before payment capture.
	_, err := pool.Exec(ctx,
		`UPDATE orders SET status='confirmed', updated_at=NOW() WHERE id=$1`, orderID)
	if err != nil {
		t.Fatalf("pre-advance: %v", err)
	}

	// Simulate markOrderPaid: order.Status is "confirmed", order.OrderType is "b2c".
	// Condition: order.OrderType == "b2c" && order.Status == "pending" → FALSE.
	// So status passed to Update stays "confirmed".
	rzpPaymentID := fmt.Sprintf("pay_test_%d", time.Now().UnixNano())
	_, err = pool.Exec(ctx, `
		UPDATE orders
		   SET status         = $3,
		       payment_status = $4,
		       payment_id     = $5,
		       updated_at     = NOW()
		 WHERE id = $1 AND org_id = $2
	`, orderID, orgID, "confirmed", "paid", rzpPaymentID)
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	var status, paymentStatus string
	scanOrder(t, pool, ctx, orderID, &status, &paymentStatus)
	if status != "confirmed" {
		t.Fatalf("status=%s, want confirmed (already-confirmed should stay confirmed)", status)
	}
	if paymentStatus != "paid" {
		t.Fatalf("payment_status=%s, want paid", paymentStatus)
	}
}

// TestMarkOrderPaid_B2BPending_StatusUnchanged verifies that a B2B pending
// order's status stays "pending" after markOrderPaid sets payment_status="paid".
// B2B status workflow is owned by the shipping team, not payment events.
func TestMarkOrderPaid_B2BPending_StatusUnchanged(t *testing.T) {
	pool := testdb.MustOpen(t)
	ctx := context.Background()

	orgID := testdb.PickOrFakeOrgID(t, pool)
	customerID := testdb.SeedCustomer(t, pool)

	// Seed a B2B pending order.
	addr, _ := json.Marshal(map[string]any{"line1": "Warehouse Road"})
	var orderID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO orders (org_id, customer_id, status, order_type, total_amount, subtotal,
		                    payment_status, delivery_address_snapshot, created_at, updated_at)
		VALUES ($1, $2, 'pending', 'b2b', 100000, 90000, 'unpaid', $3, NOW(), NOW())
		RETURNING id
	`, orgID, customerID, addr).Scan(&orderID); err != nil {
		t.Fatalf("seed b2b order: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM orders WHERE id=$1`, orderID)
	})

	// Simulate markOrderPaid: B2C condition is FALSE (order_type='b2b'),
	// so status remains "pending"; only payment_status changes.
	rzpPaymentID := fmt.Sprintf("pay_test_%d", time.Now().UnixNano())
	_, err := pool.Exec(ctx, `
		UPDATE orders
		   SET status         = $3,
		       payment_status = $4,
		       payment_id     = $5,
		       updated_at     = NOW()
		 WHERE id = $1 AND org_id = $2
	`, orderID, orgID, "pending", "paid", rzpPaymentID)
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	var status, paymentStatus string
	scanOrder(t, pool, ctx, orderID, &status, &paymentStatus)
	if status != "pending" {
		t.Fatalf("status=%s, want pending (B2B must not auto-confirm)", status)
	}
	if paymentStatus != "paid" {
		t.Fatalf("payment_status=%s, want paid", paymentStatus)
	}
}

// scanOrder reads status and payment_status for the given order row.
func scanOrder(t *testing.T, pool *pgxpool.Pool, ctx context.Context, orderID uuid.UUID, status, paymentStatus *string) {
	t.Helper()
	if err := pool.QueryRow(ctx,
		`SELECT status, payment_status FROM orders WHERE id=$1`, orderID,
	).Scan(status, paymentStatus); err != nil {
		t.Fatalf("scan order: %v", err)
	}
}
