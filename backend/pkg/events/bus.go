package events

import "context"

// Bus is the publish/subscribe interface used across the codebase.
// Backed by Valkey when available, falls back to in-process when not.
type Bus interface {
	// Publish fans an event out to everyone subscribed to the matching org.
	Publish(ctx context.Context, evt Event) error

	// Subscribe returns a channel that receives every event for the given org.
	// Caller must invoke the returned cancel func to release Valkey resources.
	Subscribe(ctx context.Context, orgID string) (<-chan Event, func(), error)

	// Close releases the underlying transport (Valkey connection).
	Close() error
}
