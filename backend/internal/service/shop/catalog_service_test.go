package shop_test

import (
	"context"
	"testing"

	"github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/internal/testdb"
	"github.com/TarunVishwakarma1/ims/backend/pkg/cache"
)

func TestCatalog_ListCategories_OnlyShopVisible(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.PickOrFakeOrgID(t, pool)
	testdb.SeedShopCategory(t, pool, orgID, "Snacks", "snacks", 1, true)
	testdb.SeedShopCategory(t, pool, orgID, "Bakery", "bakery", 2, true)
	testdb.SeedShopCategory(t, pool, orgID, "Hidden", "hidden", 99, false)

	svc := shop.NewCatalogService(pool, cache.NoOp(), orgID)
	cats, err := svc.ListCategories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cats) != 2 {
		t.Fatalf("expected 2, got %d", len(cats))
	}
	if cats[0].Slug != "snacks" || cats[1].Slug != "bakery" {
		t.Fatalf("wrong sort order: %+v", cats)
	}
}
