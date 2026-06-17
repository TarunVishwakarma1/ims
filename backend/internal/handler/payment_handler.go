package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/service"
	"github.com/TarunVishwakarma1/ims/backend/pkg/metrics"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type PaymentHandler struct {
	service                 service.PaymentService
	mockMode                bool
	liveMode                bool
	keyID                   string
	webhookSecretPrimarySet bool
	webhookSecretPrevSet    bool
}

func NewPaymentHandler(s service.PaymentService, mockMode, liveMode bool, keyID, webhookSecret, webhookSecretPrev string) *PaymentHandler {
	return &PaymentHandler{
		service:                 s,
		mockMode:                mockMode,
		liveMode:                liveMode,
		keyID:                   keyID,
		webhookSecretPrimarySet: webhookSecret != "",
		webhookSecretPrevSet:    webhookSecretPrev != "",
	}
}

// Config — GET /api/payments/config
// Frontend-safe view of payment config. UI uses these to choose between the
// mock dialog and the real RazorPay widget, and to badge LIVE vs TEST mode.
func (h *PaymentHandler) Config(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"mock":   h.mockMode,
		"live":   h.liveMode,
		"key_id": h.keyID, // public — safe to expose
	})
}

type createPaymentReq struct {
	OrderID uuid.UUID `json:"order_id"`
	Amount  int64     `json:"amount"` // paise
}

// CreateOrder — POST /api/payments/orders
// Auth required. Reserves a RazorPay order ID for the given internal order.
func (h *PaymentHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing org context")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req createPaymentReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if req.Amount <= 0 || req.OrderID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "order_id and positive amount required")
		return
	}

	payment, err := h.service.CreateOrder(r.Context(), orgID, req.OrderID, req.Amount)
	if err != nil {
		zap.L().Error("create payment failed",
			zap.String("org_id", orgID.String()),
			zap.String("order_id", req.OrderID.String()),
			zap.Int64("amount", req.Amount),
			zap.Error(err))

		msg := err.Error()
		// Client-fixable errors → 400 with message
		if containsAny(msg, "not found", "amount mismatch", "amount must be", "amount exceeds") {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
		writeError(w, http.StatusInternalServerError, "could not create payment: "+msg)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"payment":           payment,
		"razorpay_order_id": payment.RazorpayOrderID,
		"amount":            payment.Amount,
		"currency":          payment.Currency,
		"mock":              h.mockMode,
	})
}

// MockCapture — POST /api/payments/mock/capture { "razorpay_order_id": "..." }
// Dev/test only. Disabled when mockMode = false. Org-scoped.
func (h *PaymentHandler) MockCapture(w http.ResponseWriter, r *http.Request) {
	if !h.mockMode {
		writeError(w, http.StatusForbidden, "mock mode disabled")
		return
	}
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing org context")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		RazorpayOrderID string `json:"razorpay_order_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RazorpayOrderID == "" {
		writeError(w, http.StatusBadRequest, "razorpay_order_id required")
		return
	}
	if err := h.service.MockCapture(r.Context(), orgID, req.RazorpayOrderID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "captured"})
}

// MockFail — POST /api/payments/mock/fail { "razorpay_order_id": "...", "reason": "..." }
func (h *PaymentHandler) MockFail(w http.ResponseWriter, r *http.Request) {
	if !h.mockMode {
		writeError(w, http.StatusForbidden, "mock mode disabled")
		return
	}
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing org context")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		RazorpayOrderID string `json:"razorpay_order_id"`
		Reason          string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RazorpayOrderID == "" {
		writeError(w, http.StatusBadRequest, "razorpay_order_id required")
		return
	}
	reason := req.Reason
	if reason == "" {
		reason = "user cancelled"
	}
	if err := h.service.MockFail(r.Context(), orgID, req.RazorpayOrderID, reason); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "failed"})
}

// GetPayment — GET /api/payments/{id}
func (h *PaymentHandler) GetPayment(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing org context")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid payment id")
		return
	}
	p, err := h.service.GetByID(r.Context(), id, orgID)
	if err != nil {
		writeError(w, http.StatusNotFound, "payment not found")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// RefundPayment — POST /api/payments/{id}/refund
// Body: { "amount"?: int (paise, 0 = full), "reason"?: string }
func (h *PaymentHandler) RefundPayment(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing org context")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid payment id")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		Amount int64  `json:"amount"`
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if err := h.service.Refund(r.Context(), orgID, id, req.Amount, req.Reason); err != nil {
		zap.L().Warn("refund failed",
			zap.String("payment_id", id.String()),
			zap.Int64("amount", req.Amount),
			zap.Error(err))
		msg := err.Error()
		if containsAny(msg, "not found") {
			writeError(w, http.StatusNotFound, msg)
			return
		}
		if containsAny(msg, "only captured", "exceeds", "no razorpay_payment_id", "must be non-negative") {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
		// RazorPay rejections we couldn't reconcile (e.g. partial-refund
		// conflict) → 409 Conflict, not 500. They're business-state
		// errors, not server faults.
		if containsAny(msg, "fully refunded", "already refunded", "already been refunded", "razorpay refund failed") {
			writeError(w, http.StatusConflict, msg)
			return
		}
		writeError(w, http.StatusInternalServerError, msg)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "refund submitted"})
}

// ListDLQ — GET /api/payments/webhooks/dlq
// Admin endpoint: returns failed webhook events parked in the DLQ.
func (h *PaymentHandler) ListDLQ(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.ListWebhookDLQ(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list dlq")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// WebhookSecretStatus — GET /api/payments/webhooks/secrets
// Returns visibility info ONLY (never the secret values themselves):
//   primary_set: is RAZORPAY_WEBHOOK_SECRET_* configured for current mode?
//   prev_set:    is the previous-rotation secret configured?
//   live_mode:   real money vs test sandbox
//   mock_mode:   skip RazorPay entirely
//
// UI uses this to surface "rotation window active" + warn when no rotation
// secret is configured.
func (h *PaymentHandler) WebhookSecretStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"primary_set": h.webhookSecretPrimarySet,
		"prev_set":    h.webhookSecretPrevSet,
		"live_mode":   h.liveMode,
		"mock_mode":   h.mockMode,
		"key_id":      h.keyID,
	})
}

// ListEvents — GET /api/payments/webhooks/events?status=processed
func (h *PaymentHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	list, err := h.service.ListRecentWebhooks(r.Context(), status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// GetDLQEvent — GET /api/payments/webhooks/dlq/{id}
// Returns the full webhook_event row with decrypted payload so the admin UI
// can render the raw JSON for debugging.
func (h *PaymentHandler) GetDLQEvent(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid event id")
		return
	}
	evt, err := h.service.GetWebhookEvent(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "event not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, evt)
}

// ReplayDLQ — POST /api/payments/webhooks/dlq/{id}/replay
// Admin endpoint: re-runs a single dead-lettered event through the normal
// handler chain. Used after fixing whatever bug caused the original failure.
func (h *PaymentHandler) ReplayDLQ(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid event id")
		return
	}
	if err := h.service.ReplayDLQEvent(r.Context(), id); err != nil {
		zap.L().Warn("DLQ replay failed",
			zap.String("event_id", id.String()),
			zap.Error(err))
		// Still in DLQ; respond 409 — replay attempted, handler rejected.
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "event replayed and processed"})
}

// ListPayments — GET /api/payments
func (h *PaymentHandler) ListPayments(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing org context")
		return
	}
	list, err := h.service.List(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list payments")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// ── Webhook receiver (public, HMAC-verified) ─────────────────────────────

type WebhookHandler struct {
	service service.PaymentService
}

func NewWebhookHandler(s service.PaymentService) *WebhookHandler {
	return &WebhookHandler{service: s}
}

// RazorpayWebhook — POST /api/webhooks/razorpay
// Public. HMAC signature in X-Razorpay-Signature.
//
// Status codes:
//   400 — invalid signature / malformed payload (do NOT retry)
//   200 — accepted (processed or already-seen)
//   500 — transient processing error (RazorPay will retry)
func (h *WebhookHandler) RazorpayWebhook(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "could not read body", http.StatusBadRequest)
		return
	}
	signature := r.Header.Get("X-Razorpay-Signature")
	if signature == "" {
		http.Error(w, "missing signature", http.StatusBadRequest)
		return
	}
	// RazorPay sends the event id in the X-Razorpay-Event-Id header.
	// (The payload body has no top-level "id" field.)
	headerEventID := r.Header.Get("X-Razorpay-Event-Id")

	if err := h.service.ProcessWebhook(r.Context(), body, signature, headerEventID); err != nil {
		msg := err.Error()
		// Permanent errors — do not retry.
		if msg == "invalid signature" ||
			msg == "payload too large" ||
			containsAny(msg, "malformed", "missing event", "amount mismatch", "currency mismatch", "illegal transition", "event too old", "event timestamp") {
			zap.L().Warn("webhook rejected", zap.String("reason", msg))
			outcome := "rejected"
			if msg == "invalid signature" {
				outcome = "invalid_signature"
			} else if containsAny(msg, "malformed") {
				outcome = "malformed"
			}
			metrics.WebhooksReceivedTotal.WithLabelValues("razorpay", outcome).Inc()
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		// Transient — let RazorPay retry.
		zap.L().Error("webhook processing failed transiently", zap.Error(err))
		metrics.WebhooksReceivedTotal.WithLabelValues("razorpay", "transient_error").Inc()
		http.Error(w, "processing failed", http.StatusInternalServerError)
		return
	}

	metrics.WebhooksReceivedTotal.WithLabelValues("razorpay", "accepted").Inc()
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func containsAny(haystack string, needles ...string) bool {
	for _, n := range needles {
		if len(haystack) >= len(n) {
			for i := 0; i+len(n) <= len(haystack); i++ {
				if haystack[i:i+len(n)] == n {
					return true
				}
			}
		}
	}
	return false
}
