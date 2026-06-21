package jobs

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/TarunVishwakarma1/ims/backend/pkg/cache"
)

const popularityTTL = 35 * time.Minute // slightly longer than recompute interval (30m) so it never goes cold

// Popularity lookback is hard-coded as `INTERVAL '30 days'` in computePopularity SQL.
// Kept literal so callers don't pay the Go-duration → Postgres-interval string cast.

// StartPopularityRecompute rebuilds the per-product order-count map every interval
// for the given shop org. Returns a stop function. Safe to call multiple times.
func StartPopularityRecompute(ctx context.Context, c cache.Cache, pool *pgxpool.Pool, orgID uuid.UUID, interval time.Duration) func() {
	stop := make(chan struct{})
	var once sync.Once
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		run := func() {
			ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			counts, err := computePopularity(ctx, pool, orgID)
			if err != nil {
				zap.L().Warn("popularity: compute failed", zap.Error(err))
				return
			}
			if err := c.Set(ctx, "shop:popular:"+orgID.String(), counts, popularityTTL); err != nil {
				zap.L().Warn("popularity: cache set failed", zap.Error(err))
			}
		}
		run() // initial run
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

func computePopularity(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID) (map[uuid.UUID]int, error) {
	// popularityWindow = 30 days; hard-coded literal avoids string→interval cast overhead.
	rows, err := pool.Query(ctx, `
		SELECT oi.product_id, COUNT(*)
		  FROM order_items oi
		  JOIN orders o ON o.id = oi.order_id
		 WHERE o.org_id = $1
		   AND o.created_at > NOW() - INTERVAL '30 days'
		 GROUP BY oi.product_id
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[uuid.UUID]int{}
	for rows.Next() {
		var id uuid.UUID
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}
