package shop_test

import (
	"context"
	"testing"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/internal/testdb"
	"github.com/TarunVishwakarma1/ims/backend/pkg/cache"
)

func TestBanner_ListActive_HeroAndCarousel(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.PickOrFakeOrgID(t, pool)
	repo := repository.NewBannerRepository(pool)
	now := time.Now().UTC()

	mk := func(title string, isHero bool) {
		b, err := repo.Insert(context.Background(), &domain.Banner{
			OrgID: orgID, Title: title, Status: "published",
			StartsAt: now.Add(-1 * time.Hour), EndsAt: now.Add(48 * time.Hour),
			IsHero: isHero,
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, _ = pool.Exec(context.Background(), `DELETE FROM banners WHERE id=$1`, b.ID)
		})
	}
	mk("Hero", true)
	mk("Side A", false)
	mk("Side B", false)

	svc := shop.NewBannerService(repo, cache.NoOp(), orgID)
	got, err := svc.ListActive(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Hero == nil || got.Hero.Title != "Hero" {
		t.Fatalf("expected hero, got %+v", got.Hero)
	}
	if len(got.Carousel) != 2 {
		t.Fatalf("expected 2 carousel, got %d", len(got.Carousel))
	}
}

func TestBanner_CacheHit(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.PickOrFakeOrgID(t, pool)
	repo := repository.NewBannerRepository(pool)
	now := time.Now().UTC()
	b, err := repo.Insert(context.Background(), &domain.Banner{
		OrgID: orgID, Title: "X", Status: "published",
		StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour),
	})
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM banners WHERE id=$1`, b.ID) })

	// Use existing memCache helper from catalog tests (same package).
	memc := newMemCache()
	svc := shop.NewBannerService(repo, memc, orgID)
	a1, _ := svc.ListActive(context.Background(), "")
	a2, _ := svc.ListActive(context.Background(), "")
	if len(a1.Carousel) != len(a2.Carousel) {
		t.Fatalf("cache hit mismatch")
	}

	if err := svc.InvalidateActive(context.Background()); err != nil { t.Fatal(err) }
	// Insert one more; if cache invalidated, ListActive sees both.
	b2, _ := repo.Insert(context.Background(), &domain.Banner{
		OrgID: orgID, Title: "Y", Status: "published",
		StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour),
	})
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM banners WHERE id=$1`, b2.ID) })
	a3, _ := svc.ListActive(context.Background(), "")
	if len(a3.Carousel) != 2 {
		t.Fatalf("post-invalidate expected 2 carousel, got %d", len(a3.Carousel))
	}
}
