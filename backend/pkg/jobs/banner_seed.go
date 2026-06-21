package jobs

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/TarunVishwakarma1/ims/backend/pkg/cache"
	"github.com/TarunVishwakarma1/ims/backend/pkg/calendar"
)

const bannerSeedWindow = 30 * 24 * time.Hour

// StartBannerSeed runs the festival seed loop. Inserts a draft banner for each
// festival in the next 30 days that does not already have one. Returns an
// idempotent stop function. Logs but does not fail on per-event errors.
func StartBannerSeed(
	ctx context.Context,
	pool *pgxpool.Pool,
	c cache.Cache,
	orgID uuid.UUID,
	festivals []calendar.Festival,
	interval time.Duration,
) func() {
	stop := make(chan struct{})
	var once sync.Once
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		run := func() {
			rctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			if err := seedOnce(rctx, pool, orgID, festivals); err != nil {
				zap.L().Warn("banner seed: failed", zap.Error(err))
				return
			}
			_ = c.DeleteByPattern(rctx, "shop:banners:active:"+orgID.String()+"*")
		}
		run()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			case <-t.C:
				run()
			}
		}
	}()
	return func() { once.Do(func() { close(stop) }) }
}

func seedOnce(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, festivals []calendar.Festival) error {
	now := time.Now()
	cutoff := now.Add(bannerSeedWindow)
	for _, f := range festivals {
		year := now.Year()
		d := f.Date(year)
		if d.Before(now) {
			// Festival already passed this year; try next year.
			d = f.Date(year + 1)
			year++
		}
		if d.After(cutoff) {
			continue
		}
		eventKey := f.Key + "_" + strconv.Itoa(year)
		_, err := pool.Exec(ctx, `
			INSERT INTO banners (org_id, title, event_key, starts_at, ends_at, status)
			SELECT $1, $2, $3, $4, $5, 'draft'
			WHERE NOT EXISTS (
				SELECT 1 FROM banners WHERE org_id=$1 AND event_key=$3
			)
		`, orgID, f.Name+" Sale", eventKey,
			d.Add(-7*24*time.Hour), d.Add(24*time.Hour),
		)
		if err != nil {
			zap.L().Warn("banner seed: insert failed",
				zap.String("event_key", eventKey), zap.Error(err))
		}
	}
	return nil
}
