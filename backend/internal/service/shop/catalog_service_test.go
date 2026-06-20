package shop_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/internal/testdb"
	"github.com/TarunVishwakarma1/ims/backend/pkg/cache"
	"github.com/google/uuid"
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

func TestCatalog_ListProducts_FilterByCategory(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.PickOrFakeOrgID(t, pool)
	catA := testdb.SeedShopCategory(t, pool, orgID, "Snacks", "snacks", 1, true)
	catB := testdb.SeedShopCategory(t, pool, orgID, "Bakery", "bakery", 2, true)

	pA, _ := testdb.SeedProductWithStock(t, pool, "Snack A", 100, 5)
	pB, _ := testdb.SeedProductWithStock(t, pool, "Bake B", 100, 5)
	_, _ = pool.Exec(context.Background(), `UPDATE products SET org_id=$1, category_id=$2 WHERE id=$3`, orgID, catA, pA)
	_, _ = pool.Exec(context.Background(), `UPDATE products SET org_id=$1, category_id=$2 WHERE id=$3`, orgID, catB, pB)
	testdb.MarkProductShopVisible(t, pool, pA, "snack-a", "desc A", []string{"u1"}, nil)
	testdb.MarkProductShopVisible(t, pool, pB, "bake-b", "desc B", []string{"u2"}, nil)

	svc := shop.NewCatalogService(pool, cache.NoOp(), orgID)
	res, err := svc.ListProducts(context.Background(), shop.ProductListQuery{CategorySlug: "snacks", Limit: 24})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 1 || res.Items[0].Slug != "snack-a" {
		t.Fatalf("unexpected: %+v", res.Items)
	}
}

func TestCatalog_ListProducts_PriceRange(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.PickOrFakeOrgID(t, pool)
	cat := testdb.SeedShopCategory(t, pool, orgID, "All", "all", 1, true)

	for i, price := range []int64{1000, 5000, 9999} {
		p, _ := testdb.SeedProductWithStock(t, pool, "P"+string(rune('A'+i)), price, 5)
		_, _ = pool.Exec(context.Background(), `UPDATE products SET org_id=$1, category_id=$2 WHERE id=$3`, orgID, cat, p)
		testdb.MarkProductShopVisible(t, pool, p, "p-"+string(rune('a'+i)), "", nil, nil)
	}

	svc := shop.NewCatalogService(pool, cache.NoOp(), orgID)
	min := int64(2000)
	max := int64(8000)
	res, _ := svc.ListProducts(context.Background(), shop.ProductListQuery{PriceMinPaise: &min, PriceMaxPaise: &max, Limit: 24})
	if len(res.Items) != 1 || res.Items[0].PricePaise != 5000 {
		t.Fatalf("expected one 5000-paise item, got %+v", res.Items)
	}
}

func TestCatalog_ListProducts_InStockOnly(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.PickOrFakeOrgID(t, pool)
	pIn, _ := testdb.SeedProductWithStock(t, pool, "InStock", 500, 5)
	pOut, _ := testdb.SeedProductWithStock(t, pool, "OutStock", 500, 0)
	_, _ = pool.Exec(context.Background(), `UPDATE products SET org_id=$1 WHERE id=$2`, orgID, pIn)
	_, _ = pool.Exec(context.Background(), `UPDATE products SET org_id=$1 WHERE id=$2`, orgID, pOut)
	testdb.MarkProductShopVisible(t, pool, pIn, "in", "", nil, nil)
	testdb.MarkProductShopVisible(t, pool, pOut, "out", "", nil, nil)

	svc := shop.NewCatalogService(pool, cache.NoOp(), orgID)
	res, _ := svc.ListProducts(context.Background(), shop.ProductListQuery{InStockOnly: true, Limit: 24})
	if len(res.Items) == 0 {
		t.Fatalf("expected at least 1 in-stock item, got 0")
	}
	found := false
	for _, it := range res.Items {
		if it.AvailableQty == 0 {
			t.Fatalf("got zero-stock item: %+v", it)
		}
		if it.Slug == "in" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected in-stock product 'in' in result; got %+v", res.Items)
	}
}

func TestCatalog_ListProducts_SortPriceAsc(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.PickOrFakeOrgID(t, pool)
	for i, price := range []int64{500, 1500, 1000} {
		p, _ := testdb.SeedProductWithStock(t, pool, "S"+string(rune('A'+i)), price, 5)
		_, _ = pool.Exec(context.Background(), `UPDATE products SET org_id=$1 WHERE id=$2`, orgID, p)
		testdb.MarkProductShopVisible(t, pool, p, "s-"+string(rune('a'+i)), "", nil, nil)
	}
	svc := shop.NewCatalogService(pool, cache.NoOp(), orgID)
	res, _ := svc.ListProducts(context.Background(), shop.ProductListQuery{Sort: "price_asc", Limit: 24})
	if len(res.Items) < 3 {
		t.Fatalf("expected ≥3 items, got %d", len(res.Items))
	}
	for i := 1; i < len(res.Items); i++ {
		if res.Items[i-1].PricePaise > res.Items[i].PricePaise {
			t.Fatalf("not ascending at i=%d: %+v", i, res.Items)
		}
	}
}

func TestCatalog_ListProducts_Pagination(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.PickOrFakeOrgID(t, pool)
	for i := 0; i < 30; i++ {
		p, _ := testdb.SeedProductWithStock(t, pool, fmt.Sprintf("P%02d", i), 100, 5)
		_, _ = pool.Exec(context.Background(), `UPDATE products SET org_id=$1 WHERE id=$2`, orgID, p)
		testdb.MarkProductShopVisible(t, pool, p, fmt.Sprintf("p-%02d", i), "", nil, nil)
	}
	svc := shop.NewCatalogService(pool, cache.NoOp(), orgID)
	res, _ := svc.ListProducts(context.Background(), shop.ProductListQuery{Sort: "newest", Limit: 10, Offset: 10})
	if len(res.Items) != 10 {
		t.Fatalf("expected 10, got %d", len(res.Items))
	}
	if res.TotalCount < 30 {
		t.Fatalf("total %d < 30", res.TotalCount)
	}
}

func TestCatalog_ListProducts_SearchFTS(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.PickOrFakeOrgID(t, pool)
	pA, _ := testdb.SeedProductWithStock(t, pool, "Parle G Biscuit", 1000, 5)
	pB, _ := testdb.SeedProductWithStock(t, pool, "Brown Bread", 1000, 5)
	_, _ = pool.Exec(context.Background(), `UPDATE products SET org_id=$1 WHERE id=$2`, orgID, pA)
	_, _ = pool.Exec(context.Background(), `UPDATE products SET org_id=$1 WHERE id=$2`, orgID, pB)
	testdb.MarkProductShopVisible(t, pool, pA, "parle-g-biscuit", "", nil, nil)
	testdb.MarkProductShopVisible(t, pool, pB, "brown-bread", "", nil, nil)

	svc := shop.NewCatalogService(pool, cache.NoOp(), orgID)
	res, _ := svc.ListProducts(context.Background(), shop.ProductListQuery{Search: "biscuit", Limit: 24})
	if len(res.Items) == 0 || res.Items[0].Slug != "parle-g-biscuit" {
		t.Fatalf("expected parle-g-biscuit first, got %+v", res.Items)
	}
}

func TestCatalog_ListProducts_SearchFuzzy(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.PickOrFakeOrgID(t, pool)
	p, _ := testdb.SeedProductWithStock(t, pool, "Parle G Biscuit", 1000, 5)
	_, _ = pool.Exec(context.Background(), `UPDATE products SET org_id=$1 WHERE id=$2`, orgID, p)
	testdb.MarkProductShopVisible(t, pool, p, "parle-g-biscuit", "", nil, nil)

	svc := shop.NewCatalogService(pool, cache.NoOp(), orgID)
	res, _ := svc.ListProducts(context.Background(), shop.ProductListQuery{Search: "biskut", Limit: 24})
	if len(res.Items) == 0 {
		t.Fatalf("expected fuzzy hit on 'biskut', got 0 items")
	}
}

func TestCatalog_ListProducts_CursorRoundTrip(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.PickOrFakeOrgID(t, pool)
	ids := make([]uuid.UUID, 6)
	for i := range ids {
		p, _ := testdb.SeedProductWithStock(t, pool, fmt.Sprintf("C%02d", i), 100, 5)
		_, _ = pool.Exec(context.Background(), `UPDATE products SET org_id=$1 WHERE id=$2`, orgID, p)
		testdb.MarkProductShopVisible(t, pool, p, fmt.Sprintf("c-%02d", i), "", nil, nil)
		ids[i] = p
	}
	svc := shop.NewCatalogService(pool, cache.NoOp(), orgID)

	page1, _ := svc.ListProducts(context.Background(), shop.ProductListQuery{Sort: "newest", Limit: 3})
	if len(page1.Items) != 3 {
		t.Fatalf("page1 size: %d", len(page1.Items))
	}
	if page1.NextCursor == "" {
		t.Fatal("expected NextCursor on first page")
	}

	page2, _ := svc.ListProducts(context.Background(), shop.ProductListQuery{Sort: "newest", Limit: 3, Cursor: page1.NextCursor})
	if len(page2.Items) != 3 {
		t.Fatalf("page2 size: %d", len(page2.Items))
	}
	seen := map[uuid.UUID]bool{}
	for _, it := range page1.Items {
		seen[it.ID] = true
	}
	for _, it := range page2.Items {
		if seen[it.ID] {
			t.Fatalf("page2 item %s repeats from page1", it.ID)
		}
	}
}

func TestCatalog_GetProductBySlug_Found(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.PickOrFakeOrgID(t, pool)
	cat := testdb.SeedShopCategory(t, pool, orgID, "Snacks", "snacks", 1, true)
	p, _ := testdb.SeedProductWithStock(t, pool, "Parle G", 1000, 5)
	_, _ = pool.Exec(context.Background(), `UPDATE products SET category_id=$1 WHERE id=$2`, cat, p)
	testdb.MarkProductShopVisible(t, pool, p, "parle-g", "Classic biscuit", []string{"u1","u2"}, nil)

	svc := shop.NewCatalogService(pool, cache.NoOp(), orgID)
	d, err := svc.GetProductBySlug(context.Background(), "parle-g")
	if err != nil { t.Fatal(err) }
	if d.Slug != "parle-g" { t.Fatalf("slug wrong: %s", d.Slug) }
	if d.Description != "Classic biscuit" { t.Fatalf("desc wrong: %s", d.Description) }
	if len(d.ImageURLs) != 2 { t.Fatalf("images: %v", d.ImageURLs) }
	if d.CategoryName != "Snacks" { t.Fatalf("cat name: %s", d.CategoryName) }
}

func TestCatalog_GetProductBySlug_NotShopVisible_404(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.PickOrFakeOrgID(t, pool)
	p, _ := testdb.SeedProductWithStock(t, pool, "Hidden", 1000, 5)
	_, _ = pool.Exec(context.Background(), `UPDATE products SET shop_slug='hidden' WHERE id=$1`, p)

	svc := shop.NewCatalogService(pool, cache.NoOp(), orgID)
	d, err := svc.GetProductBySlug(context.Background(), "hidden")
	if !errors.Is(err, shop.ErrNotFound) || d != nil {
		t.Fatalf("expected ErrNotFound, got d=%v err=%v", d, err)
	}
}
