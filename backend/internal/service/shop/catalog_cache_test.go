package shop_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/internal/testdb"
	"github.com/TarunVishwakarma1/ims/backend/pkg/cache"
)

// countingCache wraps a cache and counts Get hits/misses for assertions.
type countingCache struct {
	inner        cache.Cache
	hits, misses, sets atomic.Int64
}

func (c *countingCache) Get(ctx context.Context, key string, dest any) error {
	err := c.inner.Get(ctx, key, dest)
	if err == nil {
		c.hits.Add(1)
	} else if errors.Is(err, cache.ErrMiss) {
		c.misses.Add(1)
	}
	return err
}
func (c *countingCache) Set(ctx context.Context, key string, val any, ttl time.Duration) error {
	c.sets.Add(1)
	return c.inner.Set(ctx, key, val, ttl)
}
func (c *countingCache) Delete(ctx context.Context, keys ...string) error {
	return c.inner.Delete(ctx, keys...)
}
func (c *countingCache) DeleteByPattern(ctx context.Context, p string) error {
	return c.inner.DeleteByPattern(ctx, p)
}
func (c *countingCache) Ping(ctx context.Context) error { return c.inner.Ping(ctx) }

func newMemCache() cache.Cache {
	return &memCache{store: map[string][]byte{}}
}

type memCache struct {
	mu    sync.Mutex
	store map[string][]byte
}

func (m *memCache) Get(_ context.Context, key string, dest any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.store[key]
	if !ok {
		return cache.ErrMiss
	}
	return json.Unmarshal(b, dest)
}
func (m *memCache) Set(_ context.Context, key string, val any, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, err := json.Marshal(val)
	if err != nil {
		return err
	}
	m.store[key] = b
	return nil
}
func (m *memCache) Delete(_ context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range keys {
		delete(m.store, k)
	}
	return nil
}
func (m *memCache) DeleteByPattern(_ context.Context, prefix string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cut := strings.TrimSuffix(prefix, "*")
	for k := range m.store {
		if strings.HasPrefix(k, cut) {
			delete(m.store, k)
		}
	}
	return nil
}
func (m *memCache) Ping(_ context.Context) error { return nil }

func TestCatalogCache_HitReturnsSameResult(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.FreshOrgID(t, pool)
	testdb.SeedShopCategory(t, pool, orgID, "Snacks", "snacks", 1, true)
	memc := newMemCache()
	cc := &countingCache{inner: memc}

	svc := shop.NewCatalogService(pool, cc, orgID)
	first, _ := svc.ListCategories(context.Background())
	second, _ := svc.ListCategories(context.Background())

	if cc.hits.Load() < 1 {
		t.Fatalf("expected cache hit on second call, got %+v", cc)
	}
	if len(first) != len(second) {
		t.Fatalf("results differ: %v vs %v", first, second)
	}
}

func TestCatalogCache_InvalidateCategories(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.PickOrFakeOrgID(t, pool)
	testdb.SeedShopCategory(t, pool, orgID, "X", "x", 1, true)
	memc := newMemCache()
	cc := &countingCache{inner: memc}
	svc := shop.NewCatalogService(pool, cc, orgID)

	_, _ = svc.ListCategories(context.Background()) // miss → set
	_, _ = svc.ListCategories(context.Background()) // hit
	startHits := cc.hits.Load()

	if err := svc.InvalidateCategories(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, _ = svc.ListCategories(context.Background()) // miss again

	if cc.hits.Load() != startHits {
		t.Fatalf("expected no new hits after invalidate, got %d", cc.hits.Load()-startHits)
	}
}

func TestCatalogCache_NoOpWhenCacheDown(t *testing.T) {
	pool := testdb.MustOpen(t)
	orgID := testdb.PickOrFakeOrgID(t, pool)
	svc := shop.NewCatalogService(pool, cache.NoOp(), orgID)

	if _, err := svc.ListCategories(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ListProducts(context.Background(), shop.ProductListQuery{Limit: 5}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetProductBySlug(context.Background(), "no-such-slug"); err == nil {
		t.Fatal("expected ErrNotFound")
	}
}
