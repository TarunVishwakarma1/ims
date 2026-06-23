package shop

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Sentinel errors specific to VerifyRazorpayPayment.
// Cross-customer access is collapsed into ErrOrderNotFound (declared in
// order_service.go, same package) so callers cannot enumerate whether an order
// exists for a given ID.
var (
	ErrInvalidSignature = errors.New("invalid signature")
	ErrOrderMismatch    = errors.New("razorpay order id mismatch")
	ErrAlreadyPaid      = errors.New("already paid")
	ErrPaymentNotFound  = errors.New("payment not found for order")
)

// VerifyInput carries the three Razorpay callback fields plus our internal
// order ID that we use to look up the payment row.
type VerifyInput struct {
	OrderID           uuid.UUID
	RazorpayOrderID   string
	RazorpayPaymentID string
	RazorpaySignature string // lowercase hex-encoded HMAC-SHA256
}

// VerifyResult is the successful outcome of VerifyRazorpayPayment.
type VerifyResult struct {
	OrderID       uuid.UUID `json:"order_id"`
	Status        string    `json:"status"`
	PaymentStatus string    `json:"payment_status"`
	InvoiceNumber string    `json:"invoice_number"`
}

// ShopPaymentService verifies Razorpay payment callbacks and syncs order state.
type ShopPaymentService interface {
	VerifyRazorpayPayment(ctx context.Context, customerID uuid.UUID, in VerifyInput) (*VerifyResult, error)
}

type shopPaymentService struct {
	pool        *pgxpool.Pool
	orgID       uuid.UUID
	orderRepo   repository.OrderRepository
	paymentRepo repository.PaymentRepository
	keySecret   string
	mockMode    bool
}

// NewShopPaymentService constructs a ShopPaymentService. When mockMode is true,
// HMAC verification is skipped — use this only in development / test environments.
func NewShopPaymentService(
	pool *pgxpool.Pool,
	orgID uuid.UUID,
	orderRepo repository.OrderRepository,
	paymentRepo repository.PaymentRepository,
	razorpayKeySecret string,
	mockMode bool,
) ShopPaymentService {
	return &shopPaymentService{
		pool:        pool,
		orgID:       orgID,
		orderRepo:   orderRepo,
		paymentRepo: paymentRepo,
		keySecret:   razorpayKeySecret,
		mockMode:    mockMode,
	}
}

// VerifyRazorpayPayment implements the full payment-capture flow:
//  1. Load order + assert ownership (404 for both not-found and wrong customer).
//  2. Idempotency guard — return ErrAlreadyPaid if already captured.
//  3. Load payment row, assert razorpay_order_id matches.
//  4. Verify HMAC-SHA256 signature (constant-time). Skipped in mock mode.
//  5. Flip orders.status → "confirmed", orders.payment_status → "paid",
//     payments.status → "captured" inside a single transaction with
//     a SELECT … FOR UPDATE re-check to prevent double-capture races.
//  6. Allocate an invoice number (best-effort; warning logged on failure).
func (s *shopPaymentService) VerifyRazorpayPayment(ctx context.Context, customerID uuid.UUID, in VerifyInput) (*VerifyResult, error) {
	// ── 1. Order lookup + ownership ──────────────────────────────────────────
	order, err := s.orderRepo.GetByID(ctx, in.OrderID, s.orgID)
	if err != nil {
		// ErrNotFound or any DB error → collapse to 404
		return nil, ErrOrderNotFound
	}
	if order.CustomerID == nil || *order.CustomerID != customerID {
		// Do not reveal "exists but not yours" — collapse to 404.
		return nil, ErrOrderNotFound
	}

	// ── 2. Idempotency — already paid? ───────────────────────────────────────
	if order.PaymentStatus == "paid" {
		return nil, ErrAlreadyPaid
	}

	// ── 3. Payment row + razorpay_order_id match ─────────────────────────────
	payment, err := s.paymentRepo.GetByOrderID(ctx, in.OrderID, s.orgID)
	if err != nil {
		return nil, ErrPaymentNotFound
	}
	if payment.RazorpayOrderID != in.RazorpayOrderID {
		return nil, ErrOrderMismatch
	}

	// ── 4. HMAC verify (constant-time; skipped in mock mode) ─────────────────
	if !s.mockMode {
		if err := verifySig(in.RazorpayOrderID, in.RazorpayPaymentID, in.RazorpaySignature, s.keySecret); err != nil {
			return nil, err
		}
	}

	// ── 5. Flip status inside a transaction ──────────────────────────────────
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Re-check inside the tx to guard against concurrent webhook/verify races.
	var curPayStatus string
	if err := tx.QueryRow(ctx,
		`SELECT payment_status FROM orders WHERE id = $1 FOR UPDATE`,
		in.OrderID,
	).Scan(&curPayStatus); err != nil {
		return nil, fmt.Errorf("re-check order: %w", err)
	}
	if curPayStatus == "paid" {
		return nil, ErrAlreadyPaid
	}

	now := time.Now().UTC()

	if _, err := tx.Exec(ctx, `
		UPDATE orders
		SET status = 'confirmed', payment_status = 'paid', updated_at = $1
		WHERE id = $2
	`, now, in.OrderID); err != nil {
		return nil, fmt.Errorf("update order: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE payments
		SET status = $1, razorpay_payment_id = $2, method = 'razorpay', updated_at = $3
		WHERE id = $4
	`, domain.PaymentStatusCaptured, in.RazorpayPaymentID, now, payment.ID); err != nil {
		return nil, fmt.Errorf("update payment: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	// ── 6. Allocate invoice number (post-commit; best-effort) ─────────────────
	fy := indianFYLabel(now)
	invNum, err := s.orderRepo.AllocateInvoiceNumber(ctx, in.OrderID, s.orgID, fy)
	if err != nil {
		zap.L().Warn("verify: allocate invoice number failed",
			zap.String("order_id", in.OrderID.String()),
			zap.Error(err),
		)
		invNum = ""
	}

	return &VerifyResult{
		OrderID:       in.OrderID,
		Status:        "confirmed",
		PaymentStatus: "paid",
		InvoiceNumber: invNum,
	}, nil
}

// verifySig computes HMAC-SHA256(key=secret, msg=rzpOrderID+"|"+rzpPaymentID)
// and compares it against sigHex using constant-time comparison.
func verifySig(rzpOrderID, rzpPaymentID, sigHex, secret string) error {
	sig, err := hex.DecodeString(sigHex)
	if err != nil || len(sig) != sha256.Size {
		return ErrInvalidSignature
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(rzpOrderID + "|" + rzpPaymentID))
	expected := mac.Sum(nil)
	if !hmac.Equal(sig, expected) {
		return ErrInvalidSignature
	}
	return nil
}
