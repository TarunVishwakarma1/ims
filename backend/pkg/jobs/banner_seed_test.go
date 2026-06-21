package jobs_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/testdb"
	"github.com/TarunVishwakarma1/ims/backend/pkg/cache"
	"github.com/TarunVishwakarma1/ims/backend/pkg/calendar"
	"github.com/TarunVishwakarma1/ims/backend/pkg/jobs"
)

func TestBannerSeed_InsertsDrafts(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.PickOrFakeOrgID(t, pool)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM banners WHERE org_id=$1 AND event_key LIKE '%_____'`, orgID)
	})

	// Use only republic_day (Jan 26) — date is far enough that "next 30 days"
	// may or may not match the current date. To make test deterministic,
	// override festivals to a single near-future event.
	now := time.Now()
	stub := []calendar.Festival{
		{Key: "stub", Name: "Stub Sale", Date: func(int) time.Time { return now.Add(24 * time.Hour) }},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	stop := jobs.StartBannerSeed(ctx, pool, cache.NoOp(), orgID, stub, 500*time.Millisecond)
	defer stop()
	time.Sleep(200 * time.Millisecond)

	var n int
	year := now.Year()
	if now.Add(24*time.Hour).Year() != year {
		year = now.Add(24 * time.Hour).Year()
	}
	if err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM banners WHERE org_id=$1 AND event_key=$2`,
		orgID, "stub_"+strconv.Itoa(year),
	).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 draft, got %d", n)
	}
}

func TestBannerSeed_StopIdempotent(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.PickOrFakeOrgID(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	stop := jobs.StartBannerSeed(ctx, pool, cache.NoOp(), orgID, nil, 5*time.Second)
	stop()
	stop() // must not panic
}

