package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// idempotencyTTL is how long a stored response stays valid. Most clients
// retry within seconds; 24h is a generous buffer for offline-resume scenarios.
const idempotencyTTL = 24 * time.Hour

// captureWriter buffers the response body + status so we can persist them
// after the handler runs. Writes pass through to the underlying writer.
type captureWriter struct {
	http.ResponseWriter
	body   bytes.Buffer
	status int
}

func (c *captureWriter) WriteHeader(s int) {
	c.status = s
	c.ResponseWriter.WriteHeader(s)
}

func (c *captureWriter) Write(p []byte) (int, error) {
	c.body.Write(p)
	return c.ResponseWriter.Write(p)
}

// Idempotency wraps a handler so repeats with the same X-Idempotency-Key
// header (scoped by org_id) return the cached prior response. Designed for
// POST /api/payments/orders to prevent duplicate RazorPay orders when a
// flaky network causes the client to retry.
//
// Behavior:
//   - No header → pass-through.
//   - Cached hit, matching body → replay stored status + body.
//   - Cached hit, body mismatch → 409 (key reused for a different request).
//   - Miss → run handler, store the response if 2xx.
func Idempotency(pool *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := strings.TrimSpace(r.Header.Get("X-Idempotency-Key"))
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}
			// scopeID is either an org_id or a customer_id — both are UUIDs
			// and stored in the same org_id column. Try org first; fall back
			// to customer for B2C shop routes that use RequireCustomer middleware.
			scopeID, ok := GetOrgIDFromContext(r.Context())
			if !ok {
				scopeID, ok = GetCustomerIDFromContext(r.Context())
			}
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "could not read body", http.StatusBadRequest)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))

			hashStr := requestHash(r.URL.Path, r.URL.RawQuery, body)

			cached, err := lookupIdempotent(r.Context(), pool, scopeID, key)
			if err == nil && cached != nil {
				if cached.RequestHash != hashStr {
					http.Error(w, "idempotency key reused with different payload", http.StatusConflict)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Idempotent-Replay", "true")
				w.WriteHeader(cached.StatusCode)
				_, _ = w.Write(cached.Body)
				return
			}

			cw := &captureWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(cw, r)

			if cw.status >= 200 && cw.status < 300 {
				if err := saveIdempotent(r.Context(), pool, scopeID, key, hashStr, cw.status, cw.body.Bytes()); err != nil {
					zap.L().Warn("idempotency: persist failed",
						zap.String("key", key), zap.Error(err))
				}
			}
		})
	}
}

type idempotentRecord struct {
	RequestHash string
	StatusCode  int
	Body        []byte
}

func requestHash(path, rawQuery string, body []byte) string {
	h := sha256.New()
	h.Write([]byte(path))
	h.Write([]byte{0})
	h.Write([]byte(rawQuery))
	h.Write([]byte{0})
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

// lookupIdempotent fetches a cached response keyed by (scopeID, key).
// scopeID is either an org_id or a customer_id — the column is named org_id
// but stores any UUID scope; no schema change is needed.
func lookupIdempotent(ctx context.Context, pool *pgxpool.Pool, scopeID uuid.UUID, key string) (*idempotentRecord, error) {
	rec := &idempotentRecord{}
	row := pool.QueryRow(ctx, `
		SELECT request_hash, status_code, response_body
		FROM payment_idempotency_keys
		WHERE org_id = $1 AND key = $2 AND expires_at > NOW()
	`, scopeID, key)
	if err := row.Scan(&rec.RequestHash, &rec.StatusCode, &rec.Body); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return rec, nil
}

// saveIdempotent persists a response under (scopeID, key) so future identical
// requests can be replayed without re-executing the handler.
func saveIdempotent(ctx context.Context, pool *pgxpool.Pool, scopeID uuid.UUID, key, hashStr string, status int, body []byte) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO payment_idempotency_keys (
			key, org_id, request_hash, status_code, response_body, expires_at
		) VALUES ($1, $2, $3, $4, $5, NOW() + $6::interval)
		ON CONFLICT (org_id, key) DO NOTHING
	`, key, scopeID, hashStr, status, body, idempotencyTTL.String())
	return err
}
