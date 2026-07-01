package repository_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/internal/testdb"
)

func TestShopProfileRepo_UpsertGet(t *testing.T) {
	pool := testdb.MustOpen(t)
	ctx := context.Background()
	repo := repository.NewShopProfileRepository(pool)

	// fresh org
	var orgID uuid.UUID
	slug := fmt.Sprintf("shop-%d", time.Now().UnixNano())
	if err := pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ($1,$2) RETURNING id`,
		"Repo Shop", slug,
	).Scan(&orgID); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM shop_profiles WHERE org_id=$1`, orgID)
		_, _ = pool.Exec(ctx, `DELETE FROM organizations WHERE id=$1`, orgID)
	})

	lat, lng := 18.53, 73.87
	p := &domain.ShopProfile{
		OrgID: orgID, Slug: slug, DisplayName: "Repo Shop",
		Pincodes: []string{"411001"}, Lat: &lat, Lng: &lng, IsLive: false,
	}
	if err := repo.Upsert(ctx, p); err != nil {
		t.Fatalf("upsert insert: %v", err)
	}

	got, err := repo.GetByOrg(ctx, orgID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.DisplayName != "Repo Shop" || len(got.Pincodes) != 1 || got.Lat == nil || *got.Lat != 18.53 {
		t.Fatalf("unexpected profile: %+v", got)
	}

	// update path
	p.DisplayName = "Repo Shop Renamed"
	if err := repo.Upsert(ctx, p); err != nil {
		t.Fatalf("upsert update: %v", err)
	}
	got, _ = repo.GetByOrg(ctx, orgID)
	if got.DisplayName != "Repo Shop Renamed" {
		t.Fatalf("update didn't stick: %+v", got)
	}

	taken, err := repo.SlugTakenByOther(ctx, slug, uuid.New())
	if err != nil {
		t.Fatalf("slug check: %v", err)
	}
	if !taken {
		t.Fatal("slug should be taken by another org")
	}
	self, _ := repo.SlugTakenByOther(ctx, slug, orgID)
	if self {
		t.Fatal("slug should not count as taken by its own org")
	}
}
