package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/pkg/events"
	"github.com/TarunVishwakarma1/ims/backend/pkg/metrics"
	"github.com/google/uuid"
	razorpay "github.com/razorpay/razorpay-go"
	"go.uber.org/zap"
)

// Event topics for payment lifecycle
const (
	TopicPaymentCreated  = "payment.created"
	TopicPaymentCaptured = "payment.captured"
	TopicPaymentFailed   = "payment.failed"
	TopicPaymentRefunded = "payment.refunded"
)

type PaymentService interface {
	// CreateOrder reserves a RazorPay order_id for an internal order.
	// Verifies the order belongs to `orgID` and that no captured payment
	// already exists for it. Returns the existing pending payment if one
	// already exists (idempotency).
	CreateOrder(ctx context.Context, orgID, orderID uuid.UUID, amount int64) (*domain.Payment, error)

	// MockCapture simulates a successful payment. Requires the caller's
	// orgID for ownership verification — prevents cross-org mock attacks.
	MockCapture(ctx context.Context, orgID uuid.UUID, razorpayOrderID string) error
	MockFail(ctx context.Context, orgID uuid.UUID, razorpayOrderID, reason string) error

	// ProcessWebhook is invoked by the public webhook endpoint. Verifies
	// signature, deduplicates, validates amount/currency, dispatches.
	// `eventID` is the provider's event id (RazorPay puts it in the
	// X-Razorpay-Event-Id header, not the body). Empty if not provided.
	ProcessWebhook(ctx context.Context, rawBody []byte, signature, eventID string) error

	GetByID(ctx context.Context, id, orgID uuid.UUID) (*domain.Payment, error)
	List(ctx context.Context, orgID uuid.UUID) ([]*domain.Payment, error)

	// ListWebhookDLQ returns failed-and-parked webhook events for ops review.
	ListWebhookDLQ(ctx context.Context) ([]*domain.WebhookEvent, error)
}

// Webhook events older than this window are rejected to prevent replay attacks.
const webhookMaxAge = 24 * time.Hour

// Retry / DLQ policy
const maxRetryAttempts = 5

// backoffFor returns the wait duration before the next retry.
// 1m, 5m, 15m, 1h, 6h — exponential-ish.
func backoffFor(attempt int) time.Duration {
	schedule := []time.Duration{
		1 * time.Minute,
		5 * time.Minute,
		15 * time.Minute,
		1 * time.Hour,
		6 * time.Hour,
	}
	if attempt < 1 {
		attempt = 1
	}
	if attempt > len(schedule) {
		attempt = len(schedule)
	}
	return schedule[attempt-1]
}

// isPermanentError flags errors that should not be retried regardless of attempt count.
func isPermanentError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	permanent := []string{
		"invalid signature",
		"payload too large",
		"malformed",
		"missing event",
		"amount mismatch",
		"currency mismatch",
		"illegal transition",
		"event too old",
		"event timestamp",
	}
	for _, p := range permanent {
		if containsSubstring(msg, p) {
			return true
		}
	}
	return false
}

func containsSubstring(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

type paymentService struct {
	paymentRepo       repository.PaymentRepository
	webhookRepo       repository.WebhookRepository
	orderRepo         repository.OrderRepository
	bus               events.Bus
	webhookSecret     string
	webhookSecretPrev string // accepted during rotation window
	keyID             string
	keySecret         string
	mockMode          bool
	rzpClient         *razorpay.Client
}

func NewPaymentService(
	paymentRepo repository.PaymentRepository,
	webhookRepo repository.WebhookRepository,
	orderRepo repository.OrderRepository,
	bus events.Bus,
	keyID, keySecret, webhookSecret, webhookSecretPrev string,
	mockMode bool,
) PaymentService {
	var client *razorpay.Client
	if !mockMode && keyID != "" && keySecret != "" {
		client = razorpay.NewClient(keyID, keySecret)
		zap.L().Info("razorpay client initialized", zap.String("key_id_prefix", keyID[:min(8, len(keyID))]))
	}
	if webhookSecretPrev != "" {
		zap.L().Info("webhook dual-secret active (rotation window)")
	}
	return &paymentService{
		paymentRepo:       paymentRepo,
		webhookRepo:       webhookRepo,
		orderRepo:         orderRepo,
		bus:               bus,
		webhookSecret:     webhookSecret,
		webhookSecretPrev: webhookSecretPrev,
		keyID:             keyID,
		keySecret:         keySecret,
		mockMode:          mockMode,
		rzpClient:         client,
	}
}

// ── CreateOrder ──────────────────────────────────────────────────────────

func (s *paymentService) CreateOrder(ctx context.Context, orgID, orderID uuid.UUID, amount int64) (*domain.Payment, error) {
	if amount <= 0 {
		return nil, errors.New("amount must be positive")
	}
	if amount > 100_00_00_000 { // 10 crore cap, sanity check
		return nil, errors.New("amount exceeds maximum allowed")
	}

	// Verify the order exists and belongs to the caller's org.
	order, err := s.orderRepo.GetByID(ctx, orderID, orgID)
	if err != nil {
		return nil, fmt.Errorf("order not found in your organization: %w", err)
	}
	// Amount sanity: must match the order total (prevents tampering).
	expected := order.TotalAmount + order.DeliveryFee - order.Discount
	if expected > 0 && amount != expected {
		return nil, fmt.Errorf("amount mismatch: order expects %d, got %d", expected, amount)
	}
	// Idempotency: if a non-failed payment already exists for this order, return it.
	if existing, err := s.paymentRepo.GetByOrderID(ctx, orderID, orgID); err == nil && existing != nil {
		if existing.Status == domain.PaymentStatusCreated ||
			existing.Status == domain.PaymentStatusAuthorized ||
			existing.Status == domain.PaymentStatusCaptured {
			return existing, nil
		}
	}

	var rzpOrderID string
	if s.rzpClient != nil {
		// Real RazorPay — round-trip to their API to create an order.
		// Reference: https://razorpay.com/docs/api/orders/
		data := map[string]any{
			"amount":   amount,         // paise
			"currency": "INR",
			"receipt":  orderID.String(),
			"notes": map[string]string{
				"org_id":   orgID.String(),
				"order_id": orderID.String(),
			},
		}
		body, err := s.rzpClient.Order.Create(data, nil)
		if err != nil {
			zap.L().Error("razorpay order create failed", zap.Error(err))
			return nil, fmt.Errorf("razorpay error: %w", err)
		}
		if id, ok := body["id"].(string); ok {
			rzpOrderID = id
		} else {
			return nil, errors.New("razorpay returned no order id")
		}
	} else {
		rzpOrderID = generateMockOrderID()
	}

	payment := &domain.Payment{
		ID:              uuid.New(),
		OrgID:           orgID,
		OrderID:         &orderID,
		RazorpayOrderID: rzpOrderID,
		Amount:          amount,
		Currency:        "INR",
		Status:          domain.PaymentStatusCreated,
		IsMock:          s.mockMode,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}

	if err := s.paymentRepo.Create(ctx, payment); err != nil {
		return nil, err
	}

	_ = s.bus.Publish(ctx, events.NewEvent(TopicPaymentCreated, orgID.String(), "", map[string]any{
		"id":                payment.ID,
		"razorpay_order_id": payment.RazorpayOrderID,
		"amount":            payment.Amount,
	}))
	mockLabel := "false"
	if s.mockMode {
		mockLabel = "true"
	}
	metrics.PaymentsCreatedTotal.WithLabelValues(mockLabel).Inc()

	return payment, nil
}

// ── Mock helpers ─────────────────────────────────────────────────────────

func (s *paymentService) MockCapture(ctx context.Context, orgID uuid.UUID, rzpOrderID string) error {
	payment, err := s.paymentRepo.GetByRazorpayOrderID(ctx, rzpOrderID)
	if err != nil {
		return err
	}
	// Ownership check — caller can only mock-capture their own org's payments.
	if payment.OrgID != orgID {
		return errors.New("payment not found")
	}
	if payment.Status != domain.PaymentStatusCreated {
		return fmt.Errorf("payment status is %q, cannot capture", payment.Status)
	}

	// Build a payload that mirrors RazorPay's `payment.captured` webhook.
	rzpPaymentID := generateMockPaymentID()
	payload := buildCapturedPayload(payment, rzpPaymentID, "card")
	rawBody, _ := json.Marshal(payload)

	// Sign it with our webhook secret — same path real RazorPay uses.
	signature := signHMAC(rawBody, s.webhookSecret)
	// Mock embeds the event id in body (`payload.id`) — pass empty header id.
	return s.ProcessWebhook(ctx, rawBody, signature, payload.EventID)
}

func (s *paymentService) MockFail(ctx context.Context, orgID uuid.UUID, rzpOrderID, reason string) error {
	payment, err := s.paymentRepo.GetByRazorpayOrderID(ctx, rzpOrderID)
	if err != nil {
		return err
	}
	if payment.OrgID != orgID {
		return errors.New("payment not found")
	}
	if payment.Status != domain.PaymentStatusCreated {
		return fmt.Errorf("payment status is %q, cannot fail", payment.Status)
	}

	payload := buildFailedPayload(payment, reason)
	rawBody, _ := json.Marshal(payload)
	signature := signHMAC(rawBody, s.webhookSecret)
	return s.ProcessWebhook(ctx, rawBody, signature, payload.EventID)
}

// ── ProcessWebhook ───────────────────────────────────────────────────────

func (s *paymentService) ProcessWebhook(ctx context.Context, rawBody []byte, signature, headerEventID string) error {
	// Body size guard — webhook payloads are small. Reject oversize.
	if len(rawBody) > 1<<19 { // 512 KB
		return errors.New("payload too large")
	}

	// Verify signature with primary secret. During rotation, also accept the
	// previous secret so in-flight webhooks signed with the old secret still
	// succeed. Constant-time comparison inside verifyHMAC.
	validPrimary := verifyHMAC(rawBody, signature, s.webhookSecret)
	validPrev := s.webhookSecretPrev != "" && verifyHMAC(rawBody, signature, s.webhookSecretPrev)
	if !validPrimary && !validPrev {
		return errors.New("invalid signature")
	}
	if validPrev && !validPrimary {
		zap.L().Warn("webhook accepted via previous secret — rotation window in use")
	}

	// Parse the envelope.
	var envelope rzpEnvelope
	if err := json.Unmarshal(rawBody, &envelope); err != nil {
		return fmt.Errorf("malformed webhook payload: %w", err)
	}

	// RazorPay sends event id in the X-Razorpay-Event-Id header, not the body.
	// Fall back to body `id` field for mock webhooks we generate ourselves.
	if envelope.EventID == "" {
		envelope.EventID = headerEventID
	}
	if envelope.Event == "" || envelope.EventID == "" {
		return errors.New("missing event or event_id")
	}

	// Replay window — reject events older than 24h.
	if envelope.CreatedAt > 0 {
		age := time.Since(time.Unix(envelope.CreatedAt, 0))
		if age > webhookMaxAge {
			return fmt.Errorf("event too old: %v", age)
		}
		// Also reject events from the future (clock skew tolerance ±5 min).
		if age < -5*time.Minute {
			return errors.New("event timestamp in future")
		}
	}

	// Idempotency check — RazorPay retries on 5xx, dedupe by event_id.
	exists, err := s.webhookRepo.Exists(ctx, envelope.EventID)
	if err != nil {
		return err
	}
	if exists {
		zap.L().Info("webhook event already processed, skipping",
			zap.String("event_id", envelope.EventID))
		return nil
	}

	// 4. Persist the raw event for audit before processing.
	evt := &domain.WebhookEvent{
		ID:        uuid.New(),
		Provider:  "razorpay",
		EventID:   envelope.EventID,
		EventType: envelope.Event,
		Signature: ptr(signature),
		Payload:   rawBody,
		Status:    "received",
		CreatedAt: time.Now().UTC(),
	}
	if err := s.webhookRepo.Insert(ctx, evt); err != nil {
		return err
	}

	// 5. Dispatch.
	var procErr error
	switch envelope.Event {
	case "payment.captured":
		procErr = s.handleCaptured(ctx, envelope, rawBody)
	case "payment.failed":
		procErr = s.handleFailed(ctx, envelope, rawBody)
	case "payment.refunded":
		procErr = s.handleRefunded(ctx, envelope, rawBody)
	default:
		zap.L().Info("unhandled webhook event type", zap.String("type", envelope.Event))
	}

	if procErr != nil {
		// Increment attempt counter, schedule retry or DLQ based on count.
		attempts, _ := s.webhookRepo.IncrementAttempts(ctx, evt.ID)
		if isPermanentError(procErr) || attempts >= maxRetryAttempts {
			_ = s.webhookRepo.SendToDLQ(ctx, evt.ID, procErr.Error())
			zap.L().Error("webhook permanently failed → DLQ",
				zap.String("event_id", evt.EventID),
				zap.Int("attempts", attempts),
				zap.Error(procErr))
		} else {
			next := time.Now().Add(backoffFor(attempts))
			_ = s.webhookRepo.MarkFailed(ctx, evt.ID, procErr.Error(), &next)
		}
		return procErr
	}
	return s.webhookRepo.MarkProcessed(ctx, evt.ID)
}

// canTransition enforces the payment state machine. Terminal states
// (captured, refunded, failed → captured is allowed back to nothing).
// captured cannot regress to failed; failed cannot become captured.
func canTransition(from, to string) bool {
	if from == to {
		return false // no-op (caught by idempotency anyway)
	}
	switch from {
	case domain.PaymentStatusCreated:
		// Any forward state allowed
		return true
	case domain.PaymentStatusAuthorized:
		return to == domain.PaymentStatusCaptured || to == domain.PaymentStatusFailed
	case domain.PaymentStatusCaptured:
		return to == domain.PaymentStatusRefunded
	case domain.PaymentStatusFailed, domain.PaymentStatusRefunded:
		return false // terminal
	}
	return false
}

func (s *paymentService) handleCaptured(ctx context.Context, env rzpEnvelope, raw json.RawMessage) error {
	entity := env.Payload.Payment.Entity
	payment, err := s.paymentRepo.GetByRazorpayOrderID(ctx, entity.OrderID)
	if err != nil {
		return fmt.Errorf("payment not found for rzp order %s: %w", entity.OrderID, err)
	}

	// Amount validation — webhook MUST match what we created. Defends against
	// a compromised RazorPay account or a forged payload (signature passed but
	// values mismatched).
	if entity.Amount != payment.Amount {
		return fmt.Errorf("amount mismatch: webhook=%d, payment=%d", entity.Amount, payment.Amount)
	}
	if entity.Currency != "" && entity.Currency != payment.Currency {
		return fmt.Errorf("currency mismatch: webhook=%q, payment=%q", entity.Currency, payment.Currency)
	}

	// State machine — block illegal transitions.
	if !canTransition(payment.Status, domain.PaymentStatusCaptured) {
		return fmt.Errorf("illegal transition %s → captured", payment.Status)
	}

	if err := s.paymentRepo.UpdateStatus(ctx, payment.ID,
		domain.PaymentStatusCaptured, entity.Method, entity.ID, "", raw); err != nil {
		return err
	}

	// Link to order — best effort. Failure here doesn't roll back the
	// payment capture (which already happened on RazorPay side) but logs
	// loudly so ops can reconcile.
	if payment.OrderID != nil {
		if err := s.markOrderPaid(ctx, *payment.OrderID, payment.OrgID, entity.ID); err != nil {
			zap.L().Error("CRITICAL: payment captured but order update failed",
				zap.String("payment_id", payment.ID.String()),
				zap.String("order_id", payment.OrderID.String()),
				zap.Error(err))
		}
	}

	_ = s.bus.Publish(ctx, events.NewEvent(TopicPaymentCaptured, payment.OrgID.String(), "", map[string]any{
		"payment_id":     payment.ID,
		"order_id":       payment.OrderID,
		"amount":         payment.Amount,
		"rzp_payment_id": entity.ID,
		"method":         entity.Method,
	}))
	method := entity.Method
	if method == "" {
		method = "unknown"
	}
	metrics.PaymentsCapturedTotal.WithLabelValues(method).Inc()
	return nil
}

func (s *paymentService) handleFailed(ctx context.Context, env rzpEnvelope, raw json.RawMessage) error {
	entity := env.Payload.Payment.Entity
	payment, err := s.paymentRepo.GetByRazorpayOrderID(ctx, entity.OrderID)
	if err != nil {
		return err
	}
	if !canTransition(payment.Status, domain.PaymentStatusFailed) {
		return fmt.Errorf("illegal transition %s → failed", payment.Status)
	}
	reason := entity.ErrorDescription
	if reason == "" {
		reason = "payment failed"
	}
	if err := s.paymentRepo.UpdateStatus(ctx, payment.ID,
		domain.PaymentStatusFailed, entity.Method, entity.ID, reason, raw); err != nil {
		return err
	}
	_ = s.bus.Publish(ctx, events.NewEvent(TopicPaymentFailed, payment.OrgID.String(), "", map[string]any{
		"payment_id": payment.ID,
		"reason":     reason,
	}))
	metrics.PaymentsFailedTotal.WithLabelValues(truncate(reason, 32)).Inc()
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func (s *paymentService) handleRefunded(ctx context.Context, env rzpEnvelope, raw json.RawMessage) error {
	entity := env.Payload.Payment.Entity
	payment, err := s.paymentRepo.GetByRazorpayOrderID(ctx, entity.OrderID)
	if err != nil {
		return err
	}
	if !canTransition(payment.Status, domain.PaymentStatusRefunded) {
		return fmt.Errorf("illegal transition %s → refunded", payment.Status)
	}
	if err := s.paymentRepo.UpdateStatus(ctx, payment.ID,
		domain.PaymentStatusRefunded, entity.Method, entity.ID, "", raw); err != nil {
		return err
	}
	_ = s.bus.Publish(ctx, events.NewEvent(TopicPaymentRefunded, payment.OrgID.String(), "", map[string]any{
		"payment_id": payment.ID,
	}))
	return nil
}

func (s *paymentService) markOrderPaid(ctx context.Context, orderID, orgID uuid.UUID, rzpPaymentID string) error {
	// Use OrderRepo's transaction-safe Update if available; otherwise fall back to a raw call.
	order, err := s.orderRepo.GetByID(ctx, orderID, orgID)
	if err != nil {
		return err
	}
	order.PaymentStatus = "paid"
	order.PaymentID = &rzpPaymentID
	now := time.Now().UTC()
	order.UpdatedAt = now
	return s.orderRepo.Update(ctx, order)
}

func (s *paymentService) GetByID(ctx context.Context, id, orgID uuid.UUID) (*domain.Payment, error) {
	return s.paymentRepo.GetByID(ctx, id, orgID)
}

func (s *paymentService) List(ctx context.Context, orgID uuid.UUID) ([]*domain.Payment, error) {
	return s.paymentRepo.List(ctx, orgID)
}

func (s *paymentService) ListWebhookDLQ(ctx context.Context) ([]*domain.WebhookEvent, error) {
	return s.webhookRepo.ListDLQ(ctx, 100)
}

// ── HMAC helpers ─────────────────────────────────────────────────────────

func signHMAC(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func verifyHMAC(body []byte, signature, secret string) bool {
	expected := signHMAC(body, secret)
	return hmac.Equal([]byte(expected), []byte(signature))
}

// ── RazorPay payload shapes (subset) ─────────────────────────────────────

type rzpEnvelope struct {
	Entity    string `json:"entity"`     // "event"
	AccountID string `json:"account_id"` // unused in mock
	Event     string `json:"event"`      // e.g. "payment.captured"
	EventID   string `json:"id"`         // unique per event
	Contains  []string `json:"contains"`
	Payload   struct {
		Payment struct {
			Entity rzpPaymentEntity `json:"entity"`
		} `json:"payment"`
	} `json:"payload"`
	CreatedAt int64 `json:"created_at"`
}

type rzpPaymentEntity struct {
	ID                string `json:"id"`
	Entity            string `json:"entity"`
	Amount            int64  `json:"amount"`
	Currency          string `json:"currency"`
	Status            string `json:"status"`
	OrderID           string `json:"order_id"`
	Method            string `json:"method"`
	ErrorCode         string `json:"error_code,omitempty"`
	ErrorDescription  string `json:"error_description,omitempty"`
}

// ── Builders for mock payloads ───────────────────────────────────────────

func buildCapturedPayload(p *domain.Payment, rzpPaymentID, method string) rzpEnvelope {
	return rzpEnvelope{
		Entity:  "event",
		Event:   "payment.captured",
		EventID: "evt_mock_" + uuid.NewString(),
		Contains: []string{"payment"},
		Payload: struct {
			Payment struct {
				Entity rzpPaymentEntity `json:"entity"`
			} `json:"payment"`
		}{
			Payment: struct {
				Entity rzpPaymentEntity `json:"entity"`
			}{
				Entity: rzpPaymentEntity{
					ID:       rzpPaymentID,
					Entity:   "payment",
					Amount:   p.Amount,
					Currency: p.Currency,
					Status:   "captured",
					OrderID:  p.RazorpayOrderID,
					Method:   method,
				},
			},
		},
		CreatedAt: time.Now().Unix(),
	}
}

func buildFailedPayload(p *domain.Payment, reason string) rzpEnvelope {
	return rzpEnvelope{
		Entity:  "event",
		Event:   "payment.failed",
		EventID: "evt_mock_" + uuid.NewString(),
		Contains: []string{"payment"},
		Payload: struct {
			Payment struct {
				Entity rzpPaymentEntity `json:"entity"`
			} `json:"payment"`
		}{
			Payment: struct {
				Entity rzpPaymentEntity `json:"entity"`
			}{
				Entity: rzpPaymentEntity{
					ID:               "pay_mock_" + shortID(),
					Entity:           "payment",
					Amount:           p.Amount,
					Currency:         p.Currency,
					Status:           "failed",
					OrderID:          p.RazorpayOrderID,
					ErrorCode:        "BAD_REQUEST_ERROR",
					ErrorDescription: reason,
				},
			},
		},
		CreatedAt: time.Now().Unix(),
	}
}

func generateMockOrderID() string   { return "order_mock_" + shortID() }
func generateMockPaymentID() string { return "pay_mock_" + shortID() }

func shortID() string {
	return uuid.NewString()[:14]
}

func ptr(s string) *string { return &s }
