package shop_test

import (
	"context"
	"testing"

	"github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/internal/testdb"
)

func TestAnalytics_SalesSummary(t *testing.T) {
	pool := testdb.MustOpen(t)
	ctx := context.Background()

	// Fresh org so the aggregate sees only this test's orders.
	orgID := testdb.FreshOrgID(t, pool)
	prod, _ := testdb.SeedProductWithStock(t, pool, "AnCandy", 100, 5)
	testdb.SeedOrderForProduct(t, pool, orgID, prod) // total_amount 100, confirmed, NOW
	testdb.SeedOrderForProduct(t, pool, orgID, prod)

	sum, err := shop.NewShopAnalyticsService(pool).SalesSummary(ctx, orgID, 30)
	if err != nil {
		t.Fatalf("SalesSummary: %v", err)
	}
	if sum.Orders != 2 {
		t.Fatalf("orders = %d, want 2", sum.Orders)
	}
	if sum.RevenuePaise != 200 {
		t.Fatalf("revenue = %d, want 200", sum.RevenuePaise)
	}
	if sum.AvgOrderPaise != 100 {
		t.Fatalf("avg order = %d, want 100", sum.AvgOrderPaise)
	}
	if len(sum.ByDay) != 1 {
		t.Fatalf("by_day len = %d, want 1 (both orders today)", len(sum.ByDay))
	}
}

func TestAnalytics_SalesSummary_EmptyOrg(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.FreshOrgID(t, pool)
	sum, err := shop.NewShopAnalyticsService(pool).SalesSummary(context.Background(), orgID, 30)
	if err != nil {
		t.Fatalf("SalesSummary: %v", err)
	}
	if sum.Orders != 0 || sum.RevenuePaise != 0 || sum.AvgOrderPaise != 0 || len(sum.ByDay) != 0 {
		t.Fatalf("empty org should be all-zero, got %+v", sum)
	}
}
