package shop_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/internal/testdb"
)

func TestNotifier_NotifySellerNewOrder(t *testing.T) {
	pool := testdb.MustOpen(t)
	ctx := context.Background()
	orgID := testdb.FreshOrgID(t, pool)

	// Active admin in that org with an email — the seller who should be alerted.
	email := fmt.Sprintf("seller-%d@example.com", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, `
		INSERT INTO users (org_id, name, email, password_hash, role, is_active)
		VALUES ($1, 'Seller', $2, 'x', 'admin', TRUE)`, orgID, email); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM notifications WHERE recipient=$1`, email)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE org_id=$1`, orgID)
	})

	prod, _ := testdb.SeedProductWithStock(t, pool, "SellerNotif", 5000, 5)
	orderID := testdb.SeedOrderForProduct(t, pool, orgID, prod)

	n := shop.NewShopNotifier(repository.NewNotificationRepository(pool), nil, nil, pool, "")
	n.NotifySellerNewOrder(ctx, orderID)

	var count int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications WHERE recipient=$1 AND subject LIKE 'New order%'`, email,
	).Scan(&count); err != nil {
		t.Fatalf("count notifications: %v", err)
	}
	if count != 1 {
		t.Fatalf("want 1 seller notification, got %d", count)
	}
}
