package shop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	srv "github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// stubPaySvc is a test double for srv.ShopPaymentService.
type stubPaySvc struct {
	res *srv.VerifyResult
	err error
}

func (s *stubPaySvc) VerifyRazorpayPayment(ctx context.Context, _ uuid.UUID, _ srv.VerifyInput) (*srv.VerifyResult, error) {
	return s.res, s.err
}

// withCustomerID injects a customer UUID into the request context the same way
// middleware.RequireCustomer does, using the exported ContextKeyCustomerID key.
func withCustomerID(req *http.Request, cid uuid.UUID) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), middleware.ContextKeyCustomerID, cid))
}

// validVerifyBody returns a JSON body with all required fields present and
// a non-nil OrderID so that the handler field-validation passes.
func validVerifyBody(t *testing.T) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]interface{}{
		"order_id":            uuid.New(),
		"razorpay_order_id":   "order_abc",
		"razorpay_payment_id": "pay_xyz",
		"razorpay_signature":  "sig123",
	})
	require.NoError(t, err)
	return b
}

func newPaymentHandler(stub *stubPaySvc) *PaymentHandler {
	return NewPaymentHandler(stub, nil)
}

func doVerify(t *testing.T, h *PaymentHandler, body []byte, injectCID *uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/payments/razorpay/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if injectCID != nil {
		req = withCustomerID(req, *injectCID)
	}
	rec := httptest.NewRecorder()
	h.Verify(rec, req)
	return rec
}

// ──────────────────────────────── tests ───────────────────────────────────

func TestVerify_GoodReturns200(t *testing.T) {
	cid := uuid.New()
	stub := &stubPaySvc{
		res: &srv.VerifyResult{
			OrderID:       uuid.New(),
			Status:        "confirmed",
			PaymentStatus: "paid",
			InvoiceNumber: "INV-001",
		},
	}
	rec := doVerify(t, newPaymentHandler(stub), validVerifyBody(t), &cid)

	require.Equal(t, http.StatusOK, rec.Code)

	var got srv.VerifyResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, "confirmed", got.Status)
	require.Equal(t, "paid", got.PaymentStatus)
}

func TestVerify_InvalidSignatureReturns400(t *testing.T) {
	cid := uuid.New()
	stub := &stubPaySvc{err: srv.ErrInvalidSignature}
	rec := doVerify(t, newPaymentHandler(stub), validVerifyBody(t), &cid)

	require.Equal(t, http.StatusBadRequest, rec.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "invalid_signature", body["error"])
}

func TestVerify_OrderMismatchReturns400(t *testing.T) {
	cid := uuid.New()
	stub := &stubPaySvc{err: srv.ErrOrderMismatch}
	rec := doVerify(t, newPaymentHandler(stub), validVerifyBody(t), &cid)

	require.Equal(t, http.StatusBadRequest, rec.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "order_mismatch", body["error"])
}

func TestVerify_AlreadyPaidReturns409(t *testing.T) {
	cid := uuid.New()
	stub := &stubPaySvc{err: srv.ErrAlreadyPaid}
	rec := doVerify(t, newPaymentHandler(stub), validVerifyBody(t), &cid)

	require.Equal(t, http.StatusConflict, rec.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "already_paid", body["error"])
}

func TestVerify_OrderNotFoundReturns404(t *testing.T) {
	cid := uuid.New()
	stub := &stubPaySvc{err: srv.ErrOrderNotFound}
	rec := doVerify(t, newPaymentHandler(stub), validVerifyBody(t), &cid)

	require.Equal(t, http.StatusNotFound, rec.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "order_not_found", body["error"])
}

func TestVerify_NoAuthReturns401(t *testing.T) {
	stub := &stubPaySvc{}
	// No customer ID injected into context.
	rec := doVerify(t, newPaymentHandler(stub), validVerifyBody(t), nil)

	require.Equal(t, http.StatusUnauthorized, rec.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "unauthorized", body["error"])
}

func TestVerify_InvalidBodyReturns400(t *testing.T) {
	cid := uuid.New()
	stub := &stubPaySvc{}
	rec := doVerify(t, newPaymentHandler(stub), []byte(`not-json`), &cid)

	require.Equal(t, http.StatusBadRequest, rec.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "invalid_body", body["error"])
}

// Ensure ErrPaymentNotFound also maps to 404 order_not_found.
func TestVerify_PaymentNotFoundReturns404(t *testing.T) {
	cid := uuid.New()
	stub := &stubPaySvc{err: srv.ErrPaymentNotFound}
	rec := doVerify(t, newPaymentHandler(stub), validVerifyBody(t), &cid)

	require.Equal(t, http.StatusNotFound, rec.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "order_not_found", body["error"])
}

// Ensure an unknown error maps to 500 verify_failed.
func TestVerify_UnknownErrReturns500(t *testing.T) {
	cid := uuid.New()
	stub := &stubPaySvc{err: errors.New("some database error")}
	rec := doVerify(t, newPaymentHandler(stub), validVerifyBody(t), &cid)

	require.Equal(t, http.StatusInternalServerError, rec.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "verify_failed", body["error"])
}
