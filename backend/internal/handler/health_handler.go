package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/pkg/cache"
	"github.com/jackc/pgx/v5/pgxpool"
)

type healthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
	Cache    string `json:"cache"`
}

func HealthCheck(pool *pgxpool.Pool, c cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		w.Header().Set("Content-Type", "application/json")

		dbOK := pool.Ping(ctx) == nil
		cacheOK := c.Ping(ctx) == nil

		status := "ok"
		code := http.StatusOK
		if !dbOK {
			// DB is critical → 503
			status = "error"
			code = http.StatusServiceUnavailable
		}
		// Cache failure is degraded but not fatal; report it in body but stay 200.

		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(healthResponse{
			Status:   status,
			Database: boolStatus(dbOK),
			Cache:    boolStatus(cacheOK),
		})
	}
}

func boolStatus(ok bool) string {
	if ok {
		return "ok"
	}
	return "error"
}
