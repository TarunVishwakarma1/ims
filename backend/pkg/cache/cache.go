package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// ErrMiss is returned when a key is not in the cache.
var ErrMiss = errors.New("cache miss")

// Cache is the interface used throughout the codebase. Backed by Valkey or a
// no-op (when Valkey isn't configured / unreachable).
type Cache interface {
	Get(ctx context.Context, key string, dest any) error
	Set(ctx context.Context, key string, val any, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
	DeleteByPattern(ctx context.Context, pattern string) error
	Ping(ctx context.Context) error
}

// ── Valkey implementation ────────────────────────────────────────────────

type valkeyCache struct {
	client *redis.Client
}

func New(url string) (Cache, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	// Sensible defaults for an internal API
	opts.PoolSize = 25
	opts.MinIdleConns = 5
	opts.DialTimeout = 3 * time.Second
	opts.ReadTimeout = 500 * time.Millisecond
	opts.WriteTimeout = 500 * time.Millisecond

	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return &valkeyCache{client: client}, nil
}

func (c *valkeyCache) Get(ctx context.Context, key string, dest any) error {
	raw, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return ErrMiss
		}
		return err
	}
	return json.Unmarshal(raw, dest)
}

func (c *valkeyCache) Set(ctx context.Context, key string, val any, ttl time.Duration) error {
	raw, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, raw, ttl).Err()
}

func (c *valkeyCache) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return c.client.Del(ctx, keys...).Err()
}

// DeleteByPattern uses SCAN+DEL — safe for production (non-blocking, no KEYS).
func (c *valkeyCache) DeleteByPattern(ctx context.Context, pattern string) error {
	iter := c.client.Scan(ctx, 0, pattern, 200).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
		if len(keys) >= 500 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
			keys = keys[:0]
		}
	}
	if err := iter.Err(); err != nil {
		return err
	}
	if len(keys) > 0 {
		return c.client.Del(ctx, keys...).Err()
	}
	return nil
}

func (c *valkeyCache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// ── No-op fallback ────────────────────────────────────────────────────────

// Noop is a Cache implementation that does nothing. Used when Valkey is
// unavailable so callers can ignore wiring details and the API still works.
type Noop struct{}

func (Noop) Get(ctx context.Context, key string, dest any) error         { return ErrMiss }
func (Noop) Set(ctx context.Context, key string, val any, ttl time.Duration) error { return nil }
func (Noop) Delete(ctx context.Context, keys ...string) error            { return nil }
func (Noop) DeleteByPattern(ctx context.Context, pattern string) error   { return nil }
func (Noop) Ping(ctx context.Context) error                              { return nil }

// NoOp returns a cache that always misses and silently no-ops on writes. Useful for tests.
func NoOp() Cache { return &noopCache{} }

type noopCache struct{}

func (n *noopCache) Get(_ context.Context, _ string, _ any) error                   { return ErrMiss }
func (n *noopCache) Set(_ context.Context, _ string, _ any, _ time.Duration) error  { return nil }
func (n *noopCache) Delete(_ context.Context, _ ...string) error                    { return nil }
func (n *noopCache) DeleteByPattern(_ context.Context, _ string) error              { return nil }
func (n *noopCache) Ping(_ context.Context) error                                   { return nil }

// MustNew returns a real cache if Valkey is reachable, otherwise a no-op cache.
// The app always boots — caching is purely an optimization.
func MustNew(url string) Cache {
	if url == "" {
		zap.L().Warn("VALKEY_URL not set, caching disabled")
		return Noop{}
	}
	c, err := New(url)
	if err != nil {
		zap.L().Warn("valkey unavailable, caching disabled", zap.Error(err))
		return Noop{}
	}
	zap.L().Info("valkey cache connected", zap.String("url", url))
	return c
}
