package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/pkg/events"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
	"go.uber.org/zap"
)

type EventsHandler struct {
	bus events.Bus
}

func NewEventsHandler(bus events.Bus) *EventsHandler {
	return &EventsHandler{bus: bus}
}

// Stream is a long-lived SSE endpoint:
//
//	GET /api/events?topics=order,inventory
//
// Auth happens via the standard Auth middleware (JWT). The connection
// receives every event for the caller's org; ?topics= narrows to specific
// prefixes ("order" matches "order.created" and "order.status_changed").
func (h *EventsHandler) Stream(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing org context")
		return
	}

	// Topic filter
	topicsParam := r.URL.Query().Get("topics")
	var topicFilters []string
	if topicsParam != "" {
		for _, t := range strings.Split(topicsParam, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				topicFilters = append(topicFilters, t)
			}
		}
	}

	// SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	// SSE is long-lived. Clear the server's per-request WriteTimeout so the
	// connection isn't killed mid-stream. http.ResponseController walks any
	// Unwrap() chain to reach the underlying http.ResponseWriter.
	rc := http.NewResponseController(w)
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		zap.L().Debug("SetWriteDeadline unsupported on writer", zap.Error(err))
	}

	// Subscribe to Valkey for this org
	stream, cancel, err := h.bus.Subscribe(r.Context(), orgID.String())
	if err != nil {
		zap.L().Error("event subscribe failed", zap.Error(err))
		http.Error(w, "subscribe failed", http.StatusInternalServerError)
		return
	}
	defer cancel()

	// Send a ready event so the client knows the stream is alive
	writeSSE(w, "ready", `{"connected":true}`)
	flusher.Flush()

	// Heartbeat ticker — prevents intermediaries from closing the connection
	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return

		case <-heartbeat.C:
			// Comment line is an SSE-safe keepalive
			fmt.Fprintln(w, ":heartbeat")
			flusher.Flush()

		case evt, ok := <-stream:
			if !ok {
				return
			}
			if !matchesFilter(evt.Type, topicFilters) {
				continue
			}
			payload, err := json.Marshal(evt)
			if err != nil {
				continue
			}
			writeSSE(w, evt.Type, string(payload))
			flusher.Flush()
		}
	}
}

func matchesFilter(eventType string, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	for _, f := range filters {
		if strings.HasPrefix(eventType, f) {
			return true
		}
	}
	return false
}

// writeSSE emits a single SSE record. id + retry are optional and we omit them
// to keep the wire format minimal; React handles dedupe via event IDs in payload.
func writeSSE(w http.ResponseWriter, event, data string) {
	if event != "" {
		fmt.Fprintf(w, "event: %s\n", event)
	}
	for _, line := range strings.Split(data, "\n") {
		fmt.Fprintf(w, "data: %s\n", line)
	}
	fmt.Fprint(w, "\n")
}
