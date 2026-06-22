package service_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/TarunVishwakarma1/ims/backend/internal/testdb"
)

// TestRefundWebhook_FlipsCancellingOrderToCancelled verifies that the targeted
// SQL UPDATE used by handleRefundProcessed (via FinalizeRefund) correctly:
//  1. Flips status from 'cancelling' → 'cancelled' on a full refund.
//  2. Sets cancelled_at.
//  3. Is idempotent — a webhook replay affects 0 rows.
func TestRefundWebhook_FlipsCancellingOrderToCancelled(t *testing.T) {
	pool := testdb.MustOpen(t)
	ctx := context.Background()

	orgID := testdb.PickOrFakeOrgID(t, pool)
	customerID := testdb.SeedCustomer(t, pool)

	// Seed order in 'cancelling' with payment_status='paid'.
	addr, _ := json.Marshal(map[string]any{"line1": "Test"})
	var orderID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO orders (org_id, user_id, customer_id, status, total_amount, order_type,
		                    payment_status, subtotal, delivery_fee, discount,
		                    delivery_address_snapshot)
		VALUES ($1, NULL, $2, 'cancelling', 500, 'b2c', 'paid', 500, 0, 0, $3)
		RETURNING id`,
		orgID, customerID, addr,
	).Scan(&orderID); err != nil {
		t.Fatalf("seed order: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM orders WHERE id=$1`, orderID)
	})

	// Simulate the SQL that FinalizeRefund (flipCancelling=true) runs.
	cmd, err := pool.Exec(ctx, `
		UPDATE orders
		   SET payment_status = 'refunded',
		       status = CASE WHEN status = 'cancelling' THEN 'cancelled' ELSE status END,
		       cancelled_at = CASE WHEN status = 'cancelling' AND cancelled_at IS NULL
		                           THEN NOW() ELSE cancelled_at END,
		       updated_at = NOW()
		 WHERE id = $1 AND org_id = $2
		   AND (payment_status != 'refunded' OR (status = 'cancelling'))
	`, orderID, orgID)
	if err != nil {
		t.Fatal(err)
	}
	if cmd.RowsAffected() != 1 {
		t.Fatalf("expected 1 row updated, got %d", cmd.RowsAffected())
	}

	// Verify state.
	var status, paymentStatus string
	var cancelledAt *time.Time
	if err := pool.QueryRow(ctx,
		`SELECT status, payment_status, cancelled_at FROM orders WHERE id=$1`, orderID,
	).Scan(&status, &paymentStatus, &cancelledAt); err != nil {
		t.Fatal(err)
	}
	if status != "cancelled" {
		t.Fatalf("status=%s, want cancelled", status)
	}
	if paymentStatus != "refunded" {
		t.Fatalf("payment_status=%s, want refunded", paymentStatus)
	}
	if cancelledAt == nil {
		t.Fatal("cancelled_at must be set after full refund")
	}

	// Replay the same UPDATE — must be a no-op (idempotency).
	cmd2, err := pool.Exec(ctx, `
		UPDATE orders
		   SET payment_status = 'refunded',
		       status = CASE WHEN status = 'cancelling' THEN 'cancelled' ELSE status END,
		       cancelled_at = CASE WHEN status = 'cancelling' AND cancelled_at IS NULL
		                           THEN NOW() ELSE cancelled_at END,
		       updated_at = NOW()
		 WHERE id = $1 AND org_id = $2
		   AND (payment_status != 'refunded' OR (status = 'cancelling'))
	`, orderID, orgID)
	if err != nil {
		t.Fatal(err)
	}
	if cmd2.RowsAffected() != 0 {
		t.Fatalf("replay should be no-op, got %d rows affected", cmd2.RowsAffected())
	}
}
