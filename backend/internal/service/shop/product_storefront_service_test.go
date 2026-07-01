package shop_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/internal/testdb"
)

func TestProductStorefront_SetGet(t *testing.T) {
	pool := testdb.MustOpen(t)
	ctx := context.Background()
	prod, orgID := testdb.SeedProductWithStock(t, pool, "PSFVisible", 5000, 10)
	svc := shop.NewProductStorefrontService(pool)

	slug := fmt.Sprintf("psf-%d", time.Now().UnixNano())
	price := int64(4500)
	got, err := svc.Set(ctx, orgID, shop.ProductStorefront{
		ProductID: prod, ShopVisible: true, ShopSlug: slug,
		ShopPricePaise: &price, ShopDescription: "on sale", ShopImageURLs: []string{"u1"},
	})
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !got.ShopVisible || got.ShopSlug != slug || got.ShopPricePaise == nil || *got.ShopPricePaise != 4500 {
		t.Fatalf("unexpected: %+v", got)
	}
	if len(got.ShopImageURLs) != 1 || got.ShopImageURLs[0] != "u1" {
		t.Fatalf("images not saved: %+v", got.ShopImageURLs)
	}

	// Re-read is stable.
	re, err := svc.Get(ctx, orgID, prod)
	if err != nil || re.ShopSlug != slug {
		t.Fatalf("Get after Set: %v / %+v", err, re)
	}
}

func TestProductStorefront_VisibleNeedsSlug(t *testing.T) {
	pool := testdb.MustOpen(t)
	prod, orgID := testdb.SeedProductWithStock(t, pool, "PSFNoSlug", 5000, 10)
	svc := shop.NewProductStorefrontService(pool)
	_, err := svc.Set(context.Background(), orgID, shop.ProductStorefront{
		ProductID: prod, ShopVisible: true, ShopSlug: "",
	})
	if !errors.Is(err, shop.ErrShopSlugRequired) {
		t.Fatalf("want ErrShopSlugRequired, got %v", err)
	}
}

func TestProductStorefront_SlugTaken(t *testing.T) {
	pool := testdb.MustOpen(t)
	ctx := context.Background()
	pA, orgID := testdb.SeedProductWithStock(t, pool, "PSFA", 5000, 10)
	pB, _ := testdb.SeedProductWithStock(t, pool, "PSFB", 5000, 10)
	svc := shop.NewProductStorefrontService(pool)

	slug := fmt.Sprintf("dup-%d", time.Now().UnixNano())
	if _, err := svc.Set(ctx, orgID, shop.ProductStorefront{ProductID: pA, ShopVisible: true, ShopSlug: slug}); err != nil {
		t.Fatalf("set A: %v", err)
	}
	_, err := svc.Set(ctx, orgID, shop.ProductStorefront{ProductID: pB, ShopVisible: true, ShopSlug: slug})
	if !errors.Is(err, shop.ErrShopSlugTaken) {
		t.Fatalf("want ErrShopSlugTaken, got %v", err)
	}
}

func TestProductStorefront_WrongOrg_NotFound(t *testing.T) {
	pool := testdb.MustOpen(t)
	prod, _ := testdb.SeedProductWithStock(t, pool, "PSFOrg", 5000, 10)
	svc := shop.NewProductStorefrontService(pool)
	_, err := svc.Set(context.Background(), uuid.New(), shop.ProductStorefront{ProductID: prod, ShopVisible: false})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
