package shop_test

import (
	"context"
	"errors"
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

func TestBanner_CreateValidation(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.PickOrFakeOrgID(t, pool)
	repo := repository.NewBannerRepository(pool)
	svc := shop.NewBannerService(repo, cache.NoOp(), orgID)
	now := time.Now().UTC()

	// Invalid date range
	_, err := svc.Create(context.Background(), shop.BannerInput{
		Title: "Bad", StartsAt: now, EndsAt: now.Add(-1 * time.Hour),
	})
	if !errors.Is(err, shop.ErrInvalidDateRange) {
		t.Fatalf("want ErrInvalidDateRange, got %v", err)
	}

	// Invalid audience
	_, err = svc.Create(context.Background(), shop.BannerInput{
		Title: "Bad", StartsAt: now, EndsAt: now.Add(time.Hour),
		AudienceFilter: "lolwut",
	})
	if !errors.Is(err, shop.ErrInvalidAudience) {
		t.Fatalf("want ErrInvalidAudience, got %v", err)
	}
}

func TestBanner_CreateGetUpdateDelete(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.PickOrFakeOrgID(t, pool)
	repo := repository.NewBannerRepository(pool)
	svc := shop.NewBannerService(repo, cache.NoOp(), orgID)
	now := time.Now().UTC()

	created, err := svc.Create(context.Background(), shop.BannerInput{
		Title: "Diwali", StartsAt: now, EndsAt: now.Add(48 * time.Hour),
		EventKey: "diwali_2026", AudienceFilter: "all",
	})
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { _ = svc.Delete(context.Background(), created.ID) })

	got, err := svc.Get(context.Background(), created.ID)
	if err != nil { t.Fatalf("Get: %v", err) }
	if got.Title != "Diwali" { t.Fatalf("title: %s", got.Title) }

	updated, err := svc.Update(context.Background(), created.ID, shop.BannerInput{
		Title: "Diwali Mega", StartsAt: now, EndsAt: now.Add(72 * time.Hour),
		EventKey: "diwali_2026", AudienceFilter: "all", SortOrder: 5,
	})
	if err != nil { t.Fatal(err) }
	if updated.Title != "Diwali Mega" || updated.SortOrder != 5 {
		t.Fatalf("update did not apply: %+v", updated)
	}

	if err := svc.Delete(context.Background(), created.ID); err != nil { t.Fatal(err) }
	if _, err := svc.Get(context.Background(), created.ID); !errors.Is(err, shop.ErrBannerNotFound) {
		t.Fatalf("want ErrBannerNotFound after delete, got %v", err)
	}
}

func TestBanner_Publish_RequiresImage(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.PickOrFakeOrgID(t, pool)
	repo := repository.NewBannerRepository(pool)
	svc := shop.NewBannerService(repo, cache.NoOp(), orgID)
	now := time.Now().UTC()
	b, _ := svc.Create(context.Background(), shop.BannerInput{
		Title: "X", StartsAt: now, EndsAt: now.Add(time.Hour),
	})
	t.Cleanup(func() { _ = svc.Delete(context.Background(), b.ID) })

	if err := svc.Publish(context.Background(), b.ID); !errors.Is(err, shop.ErrImageRequired) {
		t.Fatalf("want ErrImageRequired, got %v", err)
	}
}

func TestBanner_Publish_HeroConflict(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.PickOrFakeOrgID(t, pool)
	repo := repository.NewBannerRepository(pool)
	svc := shop.NewBannerService(repo, cache.NoOp(), orgID)
	now := time.Now().UTC()

	a, _ := svc.Create(context.Background(), shop.BannerInput{
		Title: "Hero A", ImageURL: "/x.jpg", IsHero: true,
		StartsAt: now, EndsAt: now.Add(time.Hour),
	})
	t.Cleanup(func() { _ = svc.Delete(context.Background(), a.ID) })
	if err := svc.Publish(context.Background(), a.ID); err != nil { t.Fatal(err) }

	b, _ := svc.Create(context.Background(), shop.BannerInput{
		Title: "Hero B", ImageURL: "/y.jpg", IsHero: true,
		StartsAt: now, EndsAt: now.Add(time.Hour),
	})
	t.Cleanup(func() { _ = svc.Delete(context.Background(), b.ID) })

	if err := svc.Publish(context.Background(), b.ID); !errors.Is(err, shop.ErrHeroConflict) {
		t.Fatalf("want ErrHeroConflict, got %v", err)
	}
}

func TestBanner_Archive(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.PickOrFakeOrgID(t, pool)
	repo := repository.NewBannerRepository(pool)
	svc := shop.NewBannerService(repo, cache.NoOp(), orgID)
	now := time.Now().UTC()
	b, _ := svc.Create(context.Background(), shop.BannerInput{
		Title: "X", ImageURL: "/x.jpg", StartsAt: now, EndsAt: now.Add(time.Hour),
	})
	t.Cleanup(func() { _ = svc.Delete(context.Background(), b.ID) })
	_ = svc.Publish(context.Background(), b.ID)
	if err := svc.Archive(context.Background(), b.ID); err != nil { t.Fatal(err) }
	got, _ := svc.Get(context.Background(), b.ID)
	if got.Status != "archived" { t.Fatalf("status=%s", got.Status) }
}
