package shop

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/internal/testdb"
	"github.com/TarunVishwakarma1/ims/backend/pkg/crypto"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// newSvcForTest builds a ShopPaymentService wired to the test pool.
// A no-op Encryptor (empty key) is used for PaymentRepository.
func newSvcForTest(t *testing.T, pool *pgxpool.Pool, orgID uuid.UUID, secret string, mock bool) ShopPaymentService {
	t.Helper()
	enc, err := crypto.New(nil) // empty key → no-op encryptor
	require.NoError(t, err)
	orderRepo := repository.NewOrderRepository(pool)
	paymentRepo := repository.NewPaymentRepository(pool, enc)
	return NewShopPaymentService(pool, orgID, orderRepo, paymentRepo, secret, mock)
}

// signRzp computes the Razorpay-style HMAC-SHA256 signature.
func signRzp(rzpOrderID, rzpPaymentID, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(rzpOrderID + "|" + rzpPaymentID))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyRazorpayPayment_GoodSignature(t *testing.T) {
	ctx := context.Background()
	pool, orgID, orderID, customerID, rzpOrderID := testdb.SeedB2COrderPendingRazorpay(t)
	secret := "rzp_test_secret_xxx"
	svc := newSvcForTest(t, pool, orgID, secret, false)
	sig := signRzp(rzpOrderID, "pay_TEST123", secret)

	res, err := svc.VerifyRazorpayPayment(ctx, customerID, VerifyInput{
		OrderID:           orderID,
		RazorpayOrderID:   rzpOrderID,
		RazorpayPaymentID: "pay_TEST123",
		RazorpaySignature: sig,
	})
	require.NoError(t, err)
	require.Equal(t, "confirmed", res.Status)
	require.Equal(t, "paid", res.PaymentStatus)
	require.NotEmpty(t, res.InvoiceNumber)

	o := testdb.GetShopOrder(t, pool, orderID)
	require.Equal(t, string(domain.OrderStatusConfirmed), o.Status)
	require.Equal(t, "paid", o.PaymentStatus)
}

func TestVerifyRazorpayPayment_BadSignature(t *testing.T) {
	ctx := context.Background()
	pool, orgID, orderID, customerID, rzpOrderID := testdb.SeedB2COrderPendingRazorpay(t)
	svc := newSvcForTest(t, pool, orgID, "rzp_test_secret_xxx", false)

	_, err := svc.VerifyRazorpayPayment(ctx, customerID, VerifyInput{
		OrderID:           orderID,
		RazorpayOrderID:   rzpOrderID,
		RazorpayPaymentID: "pay_TEST123",
		RazorpaySignature: "00deadbeef",
	})
	require.ErrorIs(t, err, ErrInvalidSignature)
}

func TestVerifyRazorpayPayment_OrderMismatch(t *testing.T) {
	ctx := context.Background()
	pool, orgID, orderID, customerID, _ := testdb.SeedB2COrderPendingRazorpay(t)
	secret := "rzp_test_secret_xxx"
	svc := newSvcForTest(t, pool, orgID, secret, false)
	sig := signRzp("order_OTHER", "pay_TEST123", secret)

	_, err := svc.VerifyRazorpayPayment(ctx, customerID, VerifyInput{
		OrderID:           orderID,
		RazorpayOrderID:   "order_OTHER",
		RazorpayPaymentID: "pay_TEST123",
		RazorpaySignature: sig,
	})
	require.ErrorIs(t, err, ErrOrderMismatch)
}

func TestVerifyRazorpayPayment_AlreadyPaid(t *testing.T) {
	ctx := context.Background()
	pool, orgID, orderID, customerID, rzpOrderID := testdb.SeedB2COrderPaid(t)
	secret := "rzp_test_secret_xxx"
	svc := newSvcForTest(t, pool, orgID, secret, false)
	sig := signRzp(rzpOrderID, "pay_TEST123", secret)

	_, err := svc.VerifyRazorpayPayment(ctx, customerID, VerifyInput{
		OrderID:           orderID,
		RazorpayOrderID:   rzpOrderID,
		RazorpayPaymentID: "pay_TEST123",
		RazorpaySignature: sig,
	})
	require.ErrorIs(t, err, ErrAlreadyPaid)
}

func TestVerifyRazorpayPayment_NotOwner(t *testing.T) {
	ctx := context.Background()
	pool, orgID, orderID, _, rzpOrderID := testdb.SeedB2COrderPendingRazorpay(t)
	secret := "rzp_test_secret_xxx"
	svc := newSvcForTest(t, pool, orgID, secret, false)
	sig := signRzp(rzpOrderID, "pay_TEST123", secret)

	_, err := svc.VerifyRazorpayPayment(ctx, uuid.New(), VerifyInput{
		OrderID:           orderID,
		RazorpayOrderID:   rzpOrderID,
		RazorpayPaymentID: "pay_TEST123",
		RazorpaySignature: sig,
	})
	// Collapsed to 404 — no enumeration of "exists but not yours"
	require.ErrorIs(t, err, ErrOrderNotFound)
}

func TestVerifyRazorpayPayment_MockMode_SkipsHMAC(t *testing.T) {
	ctx := context.Background()
	pool, orgID, orderID, customerID, rzpOrderID := testdb.SeedB2COrderPendingRazorpay(t)
	svc := newSvcForTest(t, pool, orgID, "", true) // mock mode — no secret needed

	res, err := svc.VerifyRazorpayPayment(ctx, customerID, VerifyInput{
		OrderID:           orderID,
		RazorpayOrderID:   rzpOrderID,
		RazorpayPaymentID: "pay_MOCK",
		RazorpaySignature: "anything",
	})
	require.NoError(t, err)
	require.Equal(t, "confirmed", res.Status)
}
