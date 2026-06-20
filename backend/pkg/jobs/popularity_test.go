package jobs_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/TarunVishwakarma1/ims/backend/internal/testdb"
	"github.com/TarunVishwakarma1/ims/backend/pkg/cache"
	"github.com/TarunVishwakarma1/ims/backend/pkg/jobs"
)

// memCache is an in-process cache backed by a plain map, safe for concurrent use.
type memCache struct {
	mu    sync.Mutex
	store map[string][]byte
}

func newMemoryCache(_ *testing.T) cache.Cache {
	return &memCache{store: map[string][]byte{}}
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

func TestPopularity_RebuildsMap(t *testing.T) {
	pool := testdb.MustOpen(t)

	// Seed product + stock; use the orgID from that product so all rows match.
	prodID, orgID := testdb.SeedProductWithStock(t, pool, "Popular Test", 100, 5)

	// Ensure the product's org_id matches what we'll query with.
	testdb.SeedOrderForProduct(t, pool, orgID, prodID)

	memc := newMemoryCache(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stop := jobs.StartPopularityRecompute(ctx, memc, pool, orgID, 200*time.Millisecond)
	defer stop()
	time.Sleep(400 * time.Millisecond)

	var counts map[uuid.UUID]int
	if err := memc.Get(context.Background(), "shop:popular:"+orgID.String(), &counts); err != nil {
		t.Fatalf("expected cache hit, got %v", err)
	}
	if counts[prodID] == 0 {
		t.Fatalf("expected counts[%s] > 0, got %v", prodID, counts)
	}
}
