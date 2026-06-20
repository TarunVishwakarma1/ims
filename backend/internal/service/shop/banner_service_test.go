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
