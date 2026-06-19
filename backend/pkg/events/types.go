package events

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Topic names — single source of truth so producers + subscribers agree.
const (
	TopicInventoryUpdated   = "inventory.updated"
	TopicLowStockAlert      = "inventory.low_stock"
	TopicOrderCreated       = "order.created"
	TopicOrderStatusChanged = "order.status_changed"
	TopicProductCreated     = "product.created"
	TopicProductUpdated     = "product.updated"
	TopicProductDeleted     = "product.deleted"
	TopicListingCreated     = "listing.created"
	TopicListingUpdated     = "listing.updated"
	TopicListingDeleted     = "listing.deleted"
)

// Event is the wire format for every published event.
// `Data` carries the topic-specific payload, decoded by subscribers.
type Event struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	OrgID     string          `json:"org_id,omitempty"`  // org-scoped events
	UserID    string          `json:"user_id,omitempty"` // who triggered it (optional)
	Data      json.RawMessage `json:"data,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// NewEvent builds an Event with a fresh ID + timestamp.
// Marshalling `data` upfront keeps the payload type-safe at call sites.
func NewEvent(eventType, orgID, userID string, data any) Event {
	var raw json.RawMessage
	if data != nil {
		raw, _ = json.Marshal(data)
	}
	return Event{
		ID:        uuid.NewString(),
		Type:      eventType,
		OrgID:     orgID,
		UserID:    userID,
		Data:      raw,
		Timestamp: time.Now().UTC(),
	}
}
