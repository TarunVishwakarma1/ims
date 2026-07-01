package shop_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/internal/testdb"
)

func TestDirectory_ListNearby(t *testing.T) {
	pool := testdb.MustOpen(t)
	ctx := context.Background()
	svc := shop.NewShopDirectoryService(pool)

	// Two orgs + live profiles. Near shop: Pune center, 5km radius. Far shop:
	// ~15km away, 5km radius (won't cover the query point).
	mk := func(name string, lat, lng, radius float64) (uuid.UUID, string) {
		var orgID uuid.UUID
		slug := fmt.Sprintf("geo-%s-%d", name, time.Now().UnixNano())
		if err := pool.QueryRow(ctx,
			`INSERT INTO organizations (name, slug) VALUES ($1,$2) RETURNING id`, name, slug,
		).Scan(&orgID); err != nil {
			t.Fatalf("seed org: %v", err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO shop_profiles (org_id, slug, display_name, lat, lng, delivery_radius_km, is_live)
			VALUES ($1,$2,$3,$4,$5,$6,TRUE)`,
			orgID, slug, name, lat, lng, radius); err != nil {
			t.Fatalf("seed profile: %v", err)
		}
		t.Cleanup(func() {
			_, _ = pool.Exec(context.Background(), `DELETE FROM shop_profiles WHERE org_id=$1`, orgID)
			_, _ = pool.Exec(context.Background(), `DELETE FROM organizations WHERE id=$1`, orgID)
		})
		return orgID, slug
	}

	// Query point: Pune (18.5204, 73.8567).
	_, nearSlug := mk("near", 18.5250, 73.8600, 5) // ~0.6 km away, within 5 km
	mk("far", 18.6500, 73.8567, 5)                 // ~14 km away, outside its 5 km radius

	got, err := svc.ListNearby(ctx, 18.5204, 73.8567)
	if err != nil {
		t.Fatalf("ListNearby: %v", err)
	}

	var foundNear, foundFar bool
	for _, s := range got {
		if s.Slug == nearSlug {
			foundNear = true
			if s.DistanceKm == nil {
				t.Fatal("near shop missing distance_km")
			}
		}
		if s.Name == "far" {
			foundFar = true
		}
	}
	if !foundNear {
		t.Fatalf("expected near shop in results, got %+v", got)
	}
	if foundFar {
		t.Fatal("far shop is outside its delivery radius and must be excluded")
	}
}
