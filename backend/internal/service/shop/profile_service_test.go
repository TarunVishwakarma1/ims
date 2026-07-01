package shop_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/internal/testdb"
)

func newShopOrg(t *testing.T) (uuid.UUID, *domain.ShopProfile, shop.ShopProfileService) {
	t.Helper()
	pool := testdb.MustOpen(t)
	ctx := context.Background()
	var orgID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ($1,$2) RETURNING id`,
		"Svc Shop", fmt.Sprintf("svc-%d", time.Now().UnixNano()),
	).Scan(&orgID); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM shop_profiles WHERE org_id=$1`, orgID)
		_, _ = pool.Exec(ctx, `DELETE FROM organizations WHERE id=$1`, orgID)
	})
	svc := shop.NewShopProfileService(repository.NewShopProfileRepository(pool))
	return orgID, nil, svc
}

func lf(v float64) *float64 { return &v }

func TestProfileSvc_CreateThenUpdate(t *testing.T) {
	orgID, _, svc := newShopOrg(t)
	ctx := context.Background()
	slug := fmt.Sprintf("svc-%d", time.Now().UnixNano())

	p, err := svc.Upsert(ctx, orgID, shop.UpsertProfileInput{
		Slug: slug, DisplayName: "Svc Shop", Pincodes: []string{"411001"},
		Lat: lf(18.5), Lng: lf(73.8), IsLive: false,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.Slug != slug {
		t.Fatalf("slug mismatch: %+v", p)
	}

	p, err = svc.Upsert(ctx, orgID, shop.UpsertProfileInput{
		Slug: slug, DisplayName: "Svc Shop Renamed", Pincodes: []string{"411001"},
		Lat: lf(18.5), Lng: lf(73.8), IsLive: false,
	})
	if err != nil || p.DisplayName != "Svc Shop Renamed" {
		t.Fatalf("update: %v %+v", err, p)
	}
}

func TestProfileSvc_GoLiveGuard(t *testing.T) {
	orgID, _, svc := newShopOrg(t)
	ctx := context.Background()
	// is_live true but no pincodes/location → rejected
	_, err := svc.Upsert(ctx, orgID, shop.UpsertProfileInput{
		Slug: fmt.Sprintf("gl-%d", time.Now().UnixNano()), DisplayName: "GL",
		IsLive: true,
	})
	if !errors.Is(err, shop.ErrGoLiveIncomplete) {
		t.Fatalf("want ErrGoLiveIncomplete, got %v", err)
	}
}

func TestProfileSvc_SlugConflict(t *testing.T) {
	orgA, _, svc := newShopOrg(t)
	orgB, _, _ := newShopOrg(t)
	ctx := context.Background()
	slug := fmt.Sprintf("dup-%d", time.Now().UnixNano())
	if _, err := svc.Upsert(ctx, orgA, shop.UpsertProfileInput{
		Slug: slug, DisplayName: "A", Pincodes: []string{"411001"},
		Lat: lf(18.5), Lng: lf(73.8),
	}); err != nil {
		t.Fatalf("seed A: %v", err)
	}
	_, err := svc.Upsert(ctx, orgB, shop.UpsertProfileInput{
		Slug: slug, DisplayName: "B", Pincodes: []string{"411002"},
		Lat: lf(18.6), Lng: lf(73.9),
	})
	if !errors.Is(err, shop.ErrSlugTaken) {
		t.Fatalf("want ErrSlugTaken, got %v", err)
	}
}

func TestProfileSvc_SlugLockedWhenLive(t *testing.T) {
	orgID, _, svc := newShopOrg(t)
	ctx := context.Background()
	slug := fmt.Sprintf("lock-%d", time.Now().UnixNano())
	if _, err := svc.Upsert(ctx, orgID, shop.UpsertProfileInput{
		Slug: slug, DisplayName: "L", Pincodes: []string{"411001"},
		Lat: lf(18.5), Lng: lf(73.8), IsLive: true,
	}); err != nil {
		t.Fatalf("go live: %v", err)
	}
	_, err := svc.Upsert(ctx, orgID, shop.UpsertProfileInput{
		Slug: slug + "-new", DisplayName: "L", Pincodes: []string{"411001"},
		Lat: lf(18.5), Lng: lf(73.8), IsLive: true,
	})
	if !errors.Is(err, shop.ErrSlugLocked) {
		t.Fatalf("want ErrSlugLocked, got %v", err)
	}
}
