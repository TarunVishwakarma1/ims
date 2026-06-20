package shop_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/internal/testdb"
	"github.com/TarunVishwakarma1/ims/backend/pkg/cache"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// seedFreshOrg creates an isolated throwaway organization so test products
// don't mix with existing seed data in the shared DB.
func seedFreshOrg(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	slug := fmt.Sprintf("test-feed-org-%d", time.Now().UnixNano())
	var id uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO organizations (name, slug) VALUES ('FeedTestOrg', $1) RETURNING id`, slug).Scan(&id); err != nil {
		t.Fatalf("seedFreshOrg: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM organizations WHERE id=$1`, id) })
	return id
}

func TestFeed_Page1_ReturnsCategoryTier(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := seedFreshOrg(t, pool)
	cat := testdb.SeedShopCategory(t, pool, orgID, "Snacks", "snacks", 1, true)
	for i := 0; i < 5; i++ {
		p, _ := testdb.SeedProductWithStock(t, pool, fmt.Sprintf("Snack%d", i), 100, 5)
		_, _ = pool.Exec(context.Background(), `UPDATE products SET org_id=$1, category_id=$2 WHERE id=$3`, orgID, cat, p)
		testdb.MarkProductShopVisible(t, pool, p, fmt.Sprintf("snack-%d", i), "", nil, nil)
	}
	svc := shop.NewFeedService(pool, cache.NoOp(), orgID)
	pg, _ := svc.Page(context.Background(), "", "snacks", 24)
	if pg.PageInfo.Tier != "category" {
		t.Fatalf("tier: %s", pg.PageInfo.Tier)
	}
	if len(pg.Items) != 5 {
		t.Fatalf("got %d items", len(pg.Items))
	}
}

func TestFeed_TierExhausted_FallsThroughInSameCall(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := seedFreshOrg(t, pool)
	catA := testdb.SeedShopCategory(t, pool, orgID, "Snacks", "snacks", 1, true)
	// 2 items in seed category, 3 outside.
	for i := 0; i < 2; i++ {
		p, _ := testdb.SeedProductWithStock(t, pool, fmt.Sprintf("Snack%d", i), 100, 5)
		_, _ = pool.Exec(context.Background(), `UPDATE products SET org_id=$1, category_id=$2 WHERE id=$3`, orgID, catA, p)
		testdb.MarkProductShopVisible(t, pool, p, fmt.Sprintf("snack-%d", i), "", nil, nil)
	}
	for i := 0; i < 3; i++ {
		p, _ := testdb.SeedProductWithStock(t, pool, fmt.Sprintf("Other%d", i), 100, 5)
		_, _ = pool.Exec(context.Background(), `UPDATE products SET org_id=$1 WHERE id=$2`, orgID, p)
		testdb.MarkProductShopVisible(t, pool, p, fmt.Sprintf("other-%d", i), "", nil, nil)
	}
	svc := shop.NewFeedService(pool, cache.NoOp(), orgID)
	pg, _ := svc.Page(context.Background(), "", "snacks", 24)
	if len(pg.Items) < 5 {
		t.Fatalf("expected fill-through, got %d", len(pg.Items))
	}
}

func TestFeed_RandomTier_DeterministicPerSeed(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := seedFreshOrg(t, pool)
	for i := 0; i < 4; i++ {
		p, _ := testdb.SeedProductWithStock(t, pool, fmt.Sprintf("Rand%d", i), 100, 5)
		_, _ = pool.Exec(context.Background(), `UPDATE products SET org_id=$1 WHERE id=$2`, orgID, p)
		testdb.MarkProductShopVisible(t, pool, p, fmt.Sprintf("rand-%d", i), "", nil, nil)
	}
	svc := shop.NewFeedService(pool, cache.NoOp(), orgID)

	// Forge cursor that lands on the random tier deterministically.
	c1 := shop.EncodeFeedCursorForTest("random", 11, "snacks", "seed-A")
	a, _ := svc.Page(context.Background(), c1, "", 24)
	b, _ := svc.Page(context.Background(), c1, "", 24)
	if !sameOrder(a.Items, b.Items) {
		t.Fatal("same seed should be deterministic")
	}

	c2 := shop.EncodeFeedCursorForTest("random", 11, "snacks", "seed-B")
	d, _ := svc.Page(context.Background(), c2, "", 24)
	if sameOrder(a.Items, d.Items) {
		t.Fatal("different seeds should differ")
	}
}

func sameOrder(a, b []shop.ProductCard) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID {
			return false
		}
	}
	return true
}
