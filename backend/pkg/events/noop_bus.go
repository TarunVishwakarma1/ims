package events

import (
	"context"

	"go.uber.org/zap"
)

// noopBus is used when Valkey is unavailable. Publishes silently swallow
// events so the API stays functional; subscribers receive nothing.
type noopBus struct{}

func NewNoopBus() Bus {
	return &noopBus{}
}

func (noopBus) Publish(ctx context.Context, evt Event) error {
	// Trace-level only; not interesting in normal logs.
	return nil
}

func (noopBus) Subscribe(ctx context.Context, orgID string) (<-chan Event, func(), error) {
	out := make(chan Event)
	cancel := func() { close(out) }
	zap.L().Warn("event bus not configured; SSE will deliver no events")
	return out, cancel, nil
}

func (noopBus) Close() error { return nil }

// MustNew returns a Valkey-backed bus if reachable, otherwise a no-op bus.
// The API always boots — events are an enhancement, not a requirement.
func MustNew(url string) Bus {
	if url == "" {
		zap.L().Warn("VALKEY_URL not set, event bus disabled")
		return NewNoopBus()
	}
	bus, err := NewValkeyBus(url)
	if err != nil {
		zap.L().Warn("valkey event bus unavailable, disabled", zap.Error(err))
		return NewNoopBus()
	}
	zap.L().Info("event bus connected", zap.String("url", url))
	return bus
}
