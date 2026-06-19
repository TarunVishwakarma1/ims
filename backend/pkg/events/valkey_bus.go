package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Channel naming: events:org:{orgID}:{type}
//   Publishers know the orgID + type up-front.
//   Subscribers use a pattern match (PSUBSCRIBE) for "everything in this org".

func channelFor(orgID, eventType string) string {
	return fmt.Sprintf("events:org:%s:%s", orgID, eventType)
}

func patternFor(orgID string) string {
	return fmt.Sprintf("events:org:%s:*", orgID)
}

type valkeyBus struct {
	client *redis.Client
}

// NewValkeyBus wires a Bus backed by Valkey/Redis pub/sub.
// Returns the no-op bus if url is empty (dev without Valkey).
func NewValkeyBus(url string) (Bus, error) {
	if url == "" {
		return NewNoopBus(), nil
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	// Pub/sub doesn't share the cache connection pool; small pool is fine
	opts.PoolSize = 10
	client := redis.NewClient(opts)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return &valkeyBus{client: client}, nil
}

func (b *valkeyBus) Publish(ctx context.Context, evt Event) error {
	if evt.OrgID == "" {
		return errors.New("event missing org_id")
	}
	payload, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	return b.client.Publish(ctx, channelFor(evt.OrgID, evt.Type), payload).Err()
}

func (b *valkeyBus) Subscribe(ctx context.Context, orgID string) (<-chan Event, func(), error) {
	pubsub := b.client.PSubscribe(ctx, patternFor(orgID))
	// Wait for subscription to be ready
	if _, err := pubsub.Receive(ctx); err != nil {
		_ = pubsub.Close()
		return nil, nil, err
	}

	out := make(chan Event, 64) // small buffer; SSE handler drains fast
	cancel := func() {
		_ = pubsub.Close()
	}

	go func() {
		defer close(out)
		ch := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				var evt Event
				if err := json.Unmarshal([]byte(msg.Payload), &evt); err != nil {
					zap.L().Warn("event decode failed", zap.Error(err))
					continue
				}
				// Best-effort delivery; drop if subscriber is too slow
				select {
				case out <- evt:
				case <-ctx.Done():
					return
				default:
					zap.L().Warn("subscriber buffer full, dropping event",
						zap.String("type", evt.Type), zap.String("org_id", evt.OrgID))
				}
			}
		}
	}()

	return out, cancel, nil
}

func (b *valkeyBus) Close() error {
	return b.client.Close()
}
