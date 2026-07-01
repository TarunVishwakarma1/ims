package shop_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/internal/testdb"
	"github.com/TarunVishwakarma1/ims/backend/pkg/events"
)

// recordingBus records every channel passed to Subscribe.
type recordingBus struct {
	mu       sync.Mutex
	channels []string
}

func (b *recordingBus) Publish(ctx context.Context, e events.Event) error { return nil }
func (b *recordingBus) Subscribe(ctx context.Context, ch string) (<-chan events.Event, func(), error) {
	b.mu.Lock()
	b.channels = append(b.channels, ch)
	b.mu.Unlock()
	c := make(chan events.Event)
	return c, func() {}, nil
}
func (b *recordingBus) Close() error { return nil }

func TestPaymentListeners_CoverLiveShops(t *testing.T) {
	pool := testdb.MustOpen(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// seed a fresh live shop org
	var orgID uuid.UUID
	slug := fmt.Sprintf("live-%d", time.Now().UnixNano())
	if err := pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ($1,$2) RETURNING id`,
		"Live Shop", slug).Scan(&orgID); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO shop_profiles (org_id, slug, display_name, is_live) VALUES ($1,$2,$3,TRUE)`,
		orgID, slug, "Live Shop"); err != nil {
		t.Fatalf("seed profile: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM shop_profiles WHERE org_id=$1`, orgID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM organizations WHERE id=$1`, orgID)
	})

	bus := &recordingBus{}
	defaultOrg := uuid.New()
	shop.StartPaymentEventListenersForLiveShops(ctx, bus, pool, defaultOrg, &shop.ShopNotifier{})

	time.Sleep(100 * time.Millisecond) // let goroutines subscribe
	bus.mu.Lock()
	defer bus.mu.Unlock()
	var sawOrg, sawDefault bool
	for _, ch := range bus.channels {
		if ch == orgID.String() {
			sawOrg = true
		}
		if ch == defaultOrg.String() {
			sawDefault = true
		}
	}
	if !sawOrg {
		t.Fatalf("live shop org %s not subscribed; channels=%v", orgID, bus.channels)
	}
	if !sawDefault {
		t.Fatal("default org not subscribed")
	}
}
