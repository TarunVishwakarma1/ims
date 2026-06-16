package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/pkg/cache"
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
	TopicPaymentRefunded     = "payment.refunded"
	TopicPaymentRefundFailed = "payment.refund_failed"
)

// PaymentOperations is the customer-facing slice: CRUD + refunds. Most
// callers (handlers, order_service) only need this slice and shouldn't be
// coupled to the webhook / admin / reconciliation surface.
type PaymentOperations interface {
	// CreateOrder reserves a RazorPay order_id for an internal order.
	// Verifies the order belongs to `orgID` and that no captured payment
	// already exists for it. Returns the existing pending payment if one
	// already exists (idempotency).
	CreateOrder(ctx context.Context, orgID, orderID uuid.UUID, amount int64) (*domain.Payment, error)

	GetByID(ctx context.Context, id, orgID uuid.UUID) (*domain.Payment, error)
	List(ctx context.Context, orgID uuid.UUID) ([]*domain.Payment, error)

	// MockCapture simulates a successful payment. Requires the caller's
	// orgID for ownership verification — prevents cross-org mock attacks.
	MockCapture(ctx context.Context, orgID uuid.UUID, razorpayOrderID string) error
	MockFail(ctx context.Context, orgID uuid.UUID, razorpayOrderID, reason string) error

	// Refund issues a refund for a captured payment. In live mode this
	// calls RazorPay's refund API and the order's payment_status flips to
	// "refunded" when the resulting webhook lands. In mock mode the flip
	// happens inline. amount=0 means full refund.
	Refund(ctx context.Context, orgID, paymentID uuid.UUID, amount int64, reason string) error

	// RefundByOrder resolves the latest captured payment for an order and
	// refunds the given amount (0 = full). Used by the cancel flow.
	// Returns nil (no-op) when the order has no captured payment.
	RefundByOrder(ctx context.Context, orgID, orderID uuid.UUID, amount int64, reason string) error
}

// WebhookProcessor is the provider-facing slice: signature verification,
// dispatch, DLQ replay, drift reconciliation. Used by webhook handler and
// the reconciliation cron only.
type WebhookProcessor interface {
	// ProcessWebhook is invoked by the public webhook endpoint. Verifies
	// signature, deduplicates, validates amount/currency, dispatches.
	// `eventID` is the provider's event id (RazorPay puts it in the
	// X-Razorpay-Event-Id header, not the body). Empty if not provided.
	ProcessWebhook(ctx context.Context, rawBody []byte, signature, eventID string) error

	// ListWebhookDLQ returns failed-and-parked webhook events for ops review.
	ListWebhookDLQ(ctx context.Context) ([]*domain.WebhookEvent, error)

	// ReplayDLQEvent re-runs a single dead-lettered webhook through the
	// normal processing pipeline. Used by admin UI to recover from
	// transient failures after the underlying bug has been fixed.
	ReplayDLQEvent(ctx context.Context, eventID uuid.UUID) error

	// Reconcile walks captured-status payments and asks RazorPay for the
	// canonical state. If they disagree (refund happened upstream but the
	// webhook never landed), local records are flipped to match. Returns
	// the number of records corrected. Idempotent. No-op in mock mode.
	Reconcile(ctx context.Context, limit int) (corrected int, err error)
}

// PaymentService preserves the original union for callers that legitimately
// need both surfaces (main wiring, and the order auto-refund flow which
// hits Refund + needs the implementation under the same handle).
type PaymentService interface {
	PaymentOperations
	WebhookProcessor
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
	auditRepo         repository.AuditLogRepository
	bus               events.Bus
	cache             cache.Cache
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
	auditRepo repository.AuditLogRepository,
	bus events.Bus,
	c cache.Cache,
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
		auditRepo:         auditRepo,
		bus:               bus,
		cache:             c,
		webhookSecret:     webhookSecret,
		webhookSecretPrev: webhookSecretPrev,
		keyID:             keyID,
		keySecret:         keySecret,
		mockMode:          mockMode,
		rzpClient:         client,
	}
}

// writeOrderAudit writes an audit log entry on the orders entity so payment
// lifecycle (captured / refunded / refund_failed) shows up in the order
// timeline view.
func (s *paymentService) writeOrderAudit(ctx context.Context, orgID, orderID uuid.UUID, action string) {
	if s.auditRepo == nil {
		return
	}
	entry := &domain.AuditLog{
		ID:        uuid.New(),
		OrgID:     orgID,
		Action:    action,
		Entity:    "orders",
		EntityID:  orderID,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.auditRepo.Create(ctx, entry); err != nil {
		zap.L().Warn("payment audit log failed", zap.Error(err))
	}
}

// invalidateOrgOrders clears the orders cache after a payment-driven order
// mutation (payment_status flips for captured / refunded).
func (s *paymentService) invalidateOrgOrders(ctx context.Context, orgID uuid.UUID) {
	if err := s.cache.DeleteByPattern(ctx, cache.OrdersListPattern(orgID)); err != nil {
		zap.L().Warn("orders cache invalidate failed", zap.Error(err))
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
	case "payment.refunded", "refund.processed":
		// RazorPay actually fires `refund.processed`. We also accept the
		// legacy `payment.refunded` name for backward compat with mocks.
		procErr = s.handleRefundProcessed(ctx, envelope, rawBody)
	case "refund.failed":
		procErr = s.handleRefundFailed(ctx, envelope, rawBody)
	case "refund.created", "refund.speed_changed":
		// Informational only — refund is in flight. No state change yet.
		zap.L().Info("refund event noted",
			zap.String("type", envelope.Event),
			zap.String("refund_id", envelope.Payload.Refund.Entity.ID))
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
	if payment.OrderID != nil {
		s.writeOrderAudit(ctx, payment.OrgID, *payment.OrderID, "payment.failed")
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

// handleRefundProcessed handles both:
//   - `refund.processed` — real RazorPay event, payload.refund.entity populated
//   - `payment.refunded` — legacy/mock event, payload.payment.entity populated
//
// Payment lookup is by razorpay_payment_id (refund payload) or
// razorpay_order_id (payment payload) — whichever is present.
func (s *paymentService) handleRefundProcessed(ctx context.Context, env rzpEnvelope, raw json.RawMessage) error {
	refund := env.Payload.Refund.Entity
	payEnt := env.Payload.Payment.Entity

	var (
		payment *domain.Payment
		err     error
	)
	switch {
	case refund.PaymentID != "":
		payment, err = s.paymentRepo.GetByRazorpayPaymentID(ctx, refund.PaymentID)
		if err != nil {
			return fmt.Errorf("payment not found for rzp payment %s: %w", refund.PaymentID, err)
		}
	case payEnt.OrderID != "":
		payment, err = s.paymentRepo.GetByRazorpayOrderID(ctx, payEnt.OrderID)
		if err != nil {
			return fmt.Errorf("payment not found for rzp order %s: %w", payEnt.OrderID, err)
		}
	default:
		return errors.New("refund event missing both refund.payment_id and payment.order_id")
	}

	if !canTransition(payment.Status, domain.PaymentStatusRefunded) {
		// Already refunded — idempotent no-op (signature was valid, event already applied).
		if payment.Status == domain.PaymentStatusRefunded {
			zap.L().Info("payment already refunded — idempotent skip",
				zap.String("payment_id", payment.ID.String()))
			return nil
		}
		return fmt.Errorf("illegal transition %s → refunded", payment.Status)
	}

	rzpRefID := refund.ID
	if rzpRefID == "" {
		rzpRefID = payEnt.ID
	}
	method := ""
	if payment.Method != nil {
		method = *payment.Method
	}
	if err := s.paymentRepo.UpdateStatus(ctx, payment.ID,
		domain.PaymentStatusRefunded, method, rzpRefID, "", raw); err != nil {
		return err
	}

	// Flip the linked order's payment_status to "refunded" so the UI badge updates.
	if payment.OrderID != nil {
		if order, err := s.orderRepo.GetByID(ctx, *payment.OrderID, payment.OrgID); err == nil {
			order.PaymentStatus = "refunded"
			order.UpdatedAt = time.Now().UTC()
			if err := s.orderRepo.Update(ctx, order); err != nil {
				zap.L().Warn("order refund-state update failed", zap.Error(err))
			} else {
				s.invalidateOrgOrders(ctx, payment.OrgID)
				s.writeOrderAudit(ctx, payment.OrgID, *payment.OrderID, "payment.refunded")
			}
		}
	}

	_ = s.bus.Publish(ctx, events.NewEvent(TopicPaymentRefunded, payment.OrgID.String(), "", map[string]any{
		"payment_id": payment.ID,
		"order_id":   payment.OrderID,
		"refund_id":  rzpRefID,
	}))
	return nil
}

// handleRefundFailed surfaces a failed refund. We don't flip payment back —
// it's still captured. Operator must retry from UI / RazorPay dashboard.
func (s *paymentService) handleRefundFailed(ctx context.Context, env rzpEnvelope, raw json.RawMessage) error {
	refund := env.Payload.Refund.Entity
	if refund.PaymentID == "" {
		return errors.New("refund.failed missing payment_id")
	}
	payment, err := s.paymentRepo.GetByRazorpayPaymentID(ctx, refund.PaymentID)
	if err != nil {
		return fmt.Errorf("payment not found for rzp payment %s: %w", refund.PaymentID, err)
	}
	zap.L().Warn("refund failed",
		zap.String("payment_id", payment.ID.String()),
		zap.String("refund_id", refund.ID),
		zap.String("status", refund.Status))
	if payment.OrderID != nil {
		s.writeOrderAudit(ctx, payment.OrgID, *payment.OrderID, "payment.refund_failed")
	}
	_ = s.bus.Publish(ctx, events.NewEvent(TopicPaymentRefundFailed, payment.OrgID.String(), "", map[string]any{
		"payment_id": payment.ID,
		"order_id":   payment.OrderID,
		"refund_id":  refund.ID,
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
	if err := s.orderRepo.Update(ctx, order); err != nil {
		return err
	}
	s.invalidateOrgOrders(ctx, orgID)
	s.writeOrderAudit(ctx, orgID, orderID, "payment.captured")
	return nil
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

// ── Refund ───────────────────────────────────────────────────────────────

func (s *paymentService) Refund(ctx context.Context, orgID, paymentID uuid.UUID, amount int64, reason string) error {
	payment, err := s.paymentRepo.GetByID(ctx, paymentID, orgID)
	if err != nil {
		return err
	}
	if payment.Status != domain.PaymentStatusCaptured {
		return fmt.Errorf("only captured payments can be refunded (current: %s)", payment.Status)
	}
	if amount < 0 {
		return errors.New("amount must be non-negative")
	}
	if amount == 0 {
		amount = payment.Amount // default full refund
	}
	if amount > payment.Amount {
		return fmt.Errorf("refund amount %d exceeds payment amount %d", amount, payment.Amount)
	}
	if payment.RazorpayPaymentID == nil || *payment.RazorpayPaymentID == "" {
		return errors.New("payment has no razorpay_payment_id; cannot refund")
	}

	// Mock mode: skip RazorPay API, simulate the refund webhook to ourselves.
	if s.rzpClient == nil {
		envelope := buildRefundedPayload(payment, amount)
		rawBody, _ := json.Marshal(envelope)
		signature := signHMAC(rawBody, s.webhookSecret)
		return s.ProcessWebhook(ctx, rawBody, signature, envelope.EventID)
	}

	// Real RazorPay: POST /payments/{id}/refund
	data := map[string]any{
		"amount": amount,
		"speed":  "normal",
		"notes": map[string]string{
			"reason":      reason,
			"internal_id": payment.ID.String(),
		},
	}
	body, err := s.rzpClient.Payment.Refund(*payment.RazorpayPaymentID, int(amount), data, nil)
	if err != nil {
		// Drift recovery: if RazorPay says the payment is already fully
		// refunded, our DB is stale (likely a missed webhook). Reconcile
		// by fetching the canonical state from RazorPay and flipping our
		// records to match — then treat the operator's refund click as a
		// success, since the desired end-state is already achieved.
		if isAlreadyRefundedErr(err) {
			zap.L().Warn("payment already refunded at RazorPay — reconciling local state",
				zap.String("payment_id", payment.ID.String()))
			if rcErr := s.reconcileRefundedFromRazorpay(ctx, payment); rcErr != nil {
				zap.L().Error("reconciliation failed", zap.Error(rcErr))
				return fmt.Errorf("razorpay refund failed: %w", err)
			}
			return nil
		}
		return fmt.Errorf("razorpay refund failed: %w", err)
	}
	zap.L().Info("razorpay refund submitted",
		zap.String("payment_id", payment.ID.String()),
		zap.Any("rzp_response", body))

	// RazorPay will fire a `refund.processed` / `payment.refunded` webhook.
	// We don't flip status here — webhook is the source of truth.
	return nil
}

func (s *paymentService) RefundByOrder(ctx context.Context, orgID, orderID uuid.UUID, amount int64, reason string) error {
	payment, err := s.paymentRepo.GetByOrderID(ctx, orderID, orgID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil // nothing to refund — order was never paid
		}
		return err
	}
	if payment.Status != domain.PaymentStatusCaptured {
		// already refunded / failed / pending — leave it alone
		return nil
	}
	// amount == 0 here means "no refund owed under policy" — distinct from
	// the Refund() sentinel where 0 means "full refund". Skip the API call.
	if amount <= 0 {
		return nil
	}
	return s.Refund(ctx, orgID, payment.ID, amount, reason)
}

// ReplayDLQEvent fetches the raw payload and signature for a dead-lettered
// webhook event and reruns ProcessWebhook. The dlq flag is left set until
// ProcessWebhook succeeds — on success, MarkProcessed clears it.
func (s *paymentService) ReplayDLQEvent(ctx context.Context, eventID uuid.UUID) error {
	evt, raw, err := s.webhookRepo.GetByID(ctx, eventID)
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		return errors.New("dlq event has no payload to replay")
	}
	sig := ""
	if evt.Signature != nil {
		sig = *evt.Signature
	}
	// Re-runs the same code path: signature verification (already passed
	// originally — should pass again), idempotency check (will short-circuit
	// because the event_id already exists, so we bypass by reusing the
	// existing row via a custom path: clear the existing one then re-insert
	// would lose history. Instead just dispatch the event handler directly.
	var envelope rzpEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("malformed payload: %w", err)
	}
	if envelope.EventID == "" {
		envelope.EventID = evt.EventID
	}

	var procErr error
	switch envelope.Event {
	case "payment.captured":
		procErr = s.handleCaptured(ctx, envelope, raw)
	case "payment.failed":
		procErr = s.handleFailed(ctx, envelope, raw)
	case "payment.refunded", "refund.processed":
		procErr = s.handleRefundProcessed(ctx, envelope, raw)
	case "refund.failed":
		procErr = s.handleRefundFailed(ctx, envelope, raw)
	default:
		procErr = fmt.Errorf("unhandled event type: %s", envelope.Event)
	}
	if procErr != nil {
		// Don't move out of DLQ — replay failed. Just bump attempts.
		_, _ = s.webhookRepo.IncrementAttempts(ctx, evt.ID)
		return procErr
	}
	_ = sig // signature already verified at original insertion
	return s.webhookRepo.MarkProcessed(ctx, evt.ID)
}

// Reconcile checks captured payments against RazorPay's canonical state and
// corrects local drift. No-op in mock mode (no rzpClient). Walks in batches
// of `limit` from oldest to newest by created_at.
func (s *paymentService) Reconcile(ctx context.Context, limit int) (int, error) {
	if s.rzpClient == nil {
		return 0, nil // mock mode — nothing to reconcile against
	}
	if limit <= 0 {
		limit = 500
	}
	const batchSize = 100
	cursor := time.Time{} // zero → from beginning
	corrected := 0
	walked := 0

	for walked < limit {
		want := batchSize
		if remaining := limit - walked; remaining < want {
			want = remaining
		}
		payments, err := s.paymentRepo.ListByStatus(ctx, domain.PaymentStatusCaptured, want, cursor)
		if err != nil {
			return corrected, err
		}
		if len(payments) == 0 {
			break
		}

		for _, p := range payments {
			walked++
			cursor = p.CreatedAt

			if p.RazorpayPaymentID == nil || *p.RazorpayPaymentID == "" {
				continue
			}
			// Ask RazorPay for the canonical record.
			body, err := s.rzpClient.Payment.Fetch(*p.RazorpayPaymentID, nil, nil)
			if err != nil {
				zap.L().Warn("reconcile fetch failed",
					zap.String("payment_id", p.ID.String()),
					zap.Error(err))
				continue
			}
			rzpStatus, _ := body["status"].(string)
			amtRefunded, _ := body["amount_refunded"].(float64)

			// If RazorPay says fully refunded but our local copy is still
			// captured → reconcile.
			if rzpStatus == "refunded" || int64(amtRefunded) >= p.Amount {
				if err := s.reconcileRefundedFromRazorpay(ctx, p); err != nil {
					zap.L().Warn("reconcile flip failed",
						zap.String("payment_id", p.ID.String()),
						zap.Error(err))
					continue
				}
				corrected++
				zap.L().Info("reconciled payment to refunded",
					zap.String("payment_id", p.ID.String()))
			}
		}

		if len(payments) < want {
			break // last partial batch
		}
	}
	zap.L().Info("payment reconciliation pass complete",
		zap.Int("walked", walked), zap.Int("corrected", corrected))
	return corrected, nil
}

// isAlreadyRefundedErr matches the RazorPay error string for an attempted
// refund on a payment that's already refunded. RazorPay's Go SDK returns
// errors as plain strings, so substring match is the only option.
func isAlreadyRefundedErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "fully refunded") ||
		strings.Contains(msg, "already been refunded") ||
		strings.Contains(msg, "already refunded")
}

// reconcileRefundedFromRazorpay flips our local payment + order rows to
// refunded WITHOUT going through a webhook. Idempotent — safe to call
// when records are already in the target state.
func (s *paymentService) reconcileRefundedFromRazorpay(ctx context.Context, payment *domain.Payment) error {
	if payment.Status != domain.PaymentStatusRefunded {
		method := ""
		if payment.Method != nil {
			method = *payment.Method
		}
		rzpRefID := ""
		if payment.RazorpayPaymentID != nil {
			rzpRefID = *payment.RazorpayPaymentID
		}
		if err := s.paymentRepo.UpdateStatus(ctx, payment.ID,
			domain.PaymentStatusRefunded, method, rzpRefID, "", nil); err != nil {
			return err
		}
	}
	if payment.OrderID != nil {
		order, err := s.orderRepo.GetByID(ctx, *payment.OrderID, payment.OrgID)
		if err == nil && order.PaymentStatus != "refunded" {
			order.PaymentStatus = "refunded"
			order.UpdatedAt = time.Now().UTC()
			if err := s.orderRepo.Update(ctx, order); err != nil {
				return err
			}
			s.invalidateOrgOrders(ctx, payment.OrgID)
			s.writeOrderAudit(ctx, payment.OrgID, *payment.OrderID, "payment.refunded")
			_ = s.bus.Publish(ctx, events.NewEvent(TopicPaymentRefunded, payment.OrgID.String(), "", map[string]any{
				"payment_id":  payment.ID,
				"order_id":    payment.OrderID,
				"reconciled":  true,
			}))
		}
	}
	return nil
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
	Entity    string      `json:"entity"`     // "event"
	AccountID string      `json:"account_id"` // unused in mock
	Event     string      `json:"event"`      // e.g. "payment.captured"
	EventID   string      `json:"id"`         // unique per event
	Contains  []string    `json:"contains"`
	Payload   rzpPayload  `json:"payload"`
	CreatedAt int64       `json:"created_at"`
}

type rzpPayload struct {
	Payment rzpPaymentWrap `json:"payment"`
	Refund  rzpRefundWrap  `json:"refund"`
}

type rzpPaymentWrap struct {
	Entity rzpPaymentEntity `json:"entity"`
}

type rzpRefundWrap struct {
	Entity rzpRefundEntity `json:"entity"`
}

type rzpPaymentEntity struct {
	ID               string `json:"id"`
	Entity           string `json:"entity"`
	Amount           int64  `json:"amount"`
	Currency         string `json:"currency"`
	Status           string `json:"status"`
	OrderID          string `json:"order_id"`
	Method           string `json:"method"`
	ErrorCode        string `json:"error_code,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// Refund webhooks have a different payload shape than payment ones.
// `payment_id` links back to the original payment.
type rzpRefundEntity struct {
	ID        string `json:"id"`         // rfnd_xxx
	Entity    string `json:"entity"`     // "refund"
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
	Status    string `json:"status"`     // processed | failed | created
	PaymentID string `json:"payment_id"` // pay_xxx — links to payment
	Notes     map[string]string `json:"notes,omitempty"`
}

// ── Builders for mock payloads ───────────────────────────────────────────

func buildCapturedPayload(p *domain.Payment, rzpPaymentID, method string) rzpEnvelope {
	return rzpEnvelope{
		Entity:   "event",
		Event:    "payment.captured",
		EventID:  "evt_mock_" + uuid.NewString(),
		Contains: []string{"payment"},
		Payload: rzpPayload{
			Payment: rzpPaymentWrap{
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
		Entity:   "event",
		Event:    "payment.failed",
		EventID:  "evt_mock_" + uuid.NewString(),
		Contains: []string{"payment"},
		Payload: rzpPayload{
			Payment: rzpPaymentWrap{
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

func buildRefundedPayload(p *domain.Payment, amount int64) rzpEnvelope {
	rzpPaymentID := ""
	if p.RazorpayPaymentID != nil {
		rzpPaymentID = *p.RazorpayPaymentID
	}
	return rzpEnvelope{
		Entity:   "event",
		Event:    "refund.processed",
		EventID:  "evt_mock_" + uuid.NewString(),
		Contains: []string{"refund"},
		Payload: rzpPayload{
			Refund: rzpRefundWrap{
				Entity: rzpRefundEntity{
					ID:        "rfnd_mock_" + shortID(),
					Entity:    "refund",
					Amount:    amount,
					Currency:  p.Currency,
					Status:    "processed",
					PaymentID: rzpPaymentID,
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
