package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/TarunVishwakarma1/ims/backend/internal/service"
	"github.com/TarunVishwakarma1/ims/backend/pkg/metrics"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type PaymentHandler struct {
	service  service.PaymentService
	mockMode bool
	liveMode bool
	keyID    string
}

func NewPaymentHandler(s service.PaymentService, mockMode, liveMode bool, keyID string) *PaymentHandler {
	return &PaymentHandler{service: s, mockMode: mockMode, liveMode: liveMode, keyID: keyID}
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
