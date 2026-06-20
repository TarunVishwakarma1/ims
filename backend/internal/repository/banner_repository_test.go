package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/internal/testdb"
)

func TestBannerRepo_InsertGet(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.PickOrFakeOrgID(t, pool)
	now := time.Now().UTC()

	repo := repository.NewBannerRepository(pool)
	in := &domain.Banner{
		OrgID:    orgID,
		Title:    "Diwali Sale",
		EventKey: "diwali_2026",
		StartsAt: now,
		EndsAt:   now.Add(48 * time.Hour),
		Status:   "draft",
	}
	out, err := repo.Insert(context.Background(), in)
	if err != nil { t.Fatal(err) }
	if out.ID == uuid.Nil { t.Fatal("expected non-nil ID") }

	got, err := repo.GetByID(context.Background(), orgID, out.ID)
	if err != nil { t.Fatal(err) }
	if got.Title != "Diwali Sale" || got.Status != "draft" {
		t.Fatalf("unexpected: %+v", got)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM banners WHERE id=$1`, out.ID)
	})
}

func TestBannerRepo_ListActive(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.PickOrFakeOrgID(t, pool)
	now := time.Now().UTC()
	repo := repository.NewBannerRepository(pool)

	mk := func(title, status string, start, end time.Time, isHero bool) uuid.UUID {
		b, err := repo.Insert(context.Background(), &domain.Banner{
			OrgID: orgID, Title: title, Status: status,
			StartsAt: start, EndsAt: end, IsHero: isHero,
		})
		if err != nil { t.Fatal(err) }
		t.Cleanup(func() {
			_, _ = pool.Exec(context.Background(), `DELETE FROM banners WHERE id=$1`, b.ID)
		})
		return b.ID
	}

	mk("Past",      "published", now.Add(-72*time.Hour), now.Add(-1*time.Hour), false)
	mk("Future",    "published", now.Add(48*time.Hour),  now.Add(72*time.Hour), false)
	mk("Active",    "published", now.Add(-1*time.Hour),  now.Add(48*time.Hour), false)
	mk("Hero",      "published", now.Add(-1*time.Hour),  now.Add(48*time.Hour), true)
	mk("Draft",     "draft",     now.Add(-1*time.Hour),  now.Add(48*time.Hour), false)

	got, err := repo.ListActive(context.Background(), orgID, "", now)
	if err != nil { t.Fatal(err) }
	if len(got) != 2 {
		t.Fatalf("expected 2 active rows (Active + Hero), got %d: %+v", len(got), got)
	}
	if got[0].IsHero != true {
		t.Fatalf("expected hero first (ORDER BY is_hero DESC); got %+v", got[0])
	}
}

func TestBannerRepo_Update_ChangesStatus(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.PickOrFakeOrgID(t, pool)
	now := time.Now().UTC()
	repo := repository.NewBannerRepository(pool)

	b, err := repo.Insert(context.Background(), &domain.Banner{
		OrgID:    orgID,
		Title:    "Status Test",
		StartsAt: now,
		EndsAt:   now.Add(48 * time.Hour),
		Status:   "draft",
	})
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM banners WHERE id=$1`, b.ID) })

	b.Status = "published"
	b.ImageURL = "/x.jpg"
	if _, err := repo.Update(context.Background(), b); err != nil { t.Fatal(err) }

	got, err := repo.GetByID(context.Background(), orgID, b.ID)
	if err != nil { t.Fatal(err) }
	if got.Status != "published" {
		t.Fatalf("expected status=published after Update, got %q", got.Status)
	}
}
