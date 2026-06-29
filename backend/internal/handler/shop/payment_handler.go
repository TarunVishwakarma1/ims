package shop

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	srv "github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
)

// PaymentHandler handles Razorpay payment verification.
type PaymentHandler struct {
	svc      srv.ShopPaymentService
	notifier *srv.ShopNotifier // may be nil — notifications disabled
}

// NewPaymentHandler constructs a PaymentHandler backed by the given service.
// notifier may be nil to disable payment emails.
func NewPaymentHandler(s srv.ShopPaymentService, notifier *srv.ShopNotifier) *PaymentHandler {
	return &PaymentHandler{svc: s, notifier: notifier}
}

type razorpayVerifyReq struct {
	OrderID           uuid.UUID `json:"order_id"`
	RazorpayOrderID   string    `json:"razorpay_order_id"`
	RazorpayPaymentID string    `json:"razorpay_payment_id"`
	RazorpaySignature string    `json:"razorpay_signature"`
}

// Verify handles POST /api/shop/payments/razorpay/verify.
func (h *PaymentHandler) Verify(w http.ResponseWriter, r *http.Request) {
	cid, ok := middleware.GetCustomerIDFromContext(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req razorpayVerifyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if req.OrderID == uuid.Nil || req.RazorpayOrderID == "" || req.RazorpayPaymentID == "" || req.RazorpaySignature == "" {
		writeErr(w, http.StatusBadRequest, "invalid_body")
		return
	}

	res, err := h.svc.VerifyRazorpayPayment(r.Context(), cid, srv.VerifyInput{
		OrderID:           req.OrderID,
		RazorpayOrderID:   req.RazorpayOrderID,
		RazorpayPaymentID: req.RazorpayPaymentID,
		RazorpaySignature: req.RazorpaySignature,
	})
	if err != nil {
		switch {
		case errors.Is(err, srv.ErrOrderNotFound), errors.Is(err, srv.ErrPaymentNotFound):
			writeErr(w, http.StatusNotFound, "order_not_found")
		case errors.Is(err, srv.ErrInvalidSignature):
			writeErr(w, http.StatusBadRequest, "invalid_signature")
		case errors.Is(err, srv.ErrOrderMismatch):
			writeErr(w, http.StatusBadRequest, "order_mismatch")
		case errors.Is(err, srv.ErrAlreadyPaid):
			writeErr(w, http.StatusConflict, "already_paid")
		default:
			writeErr(w, http.StatusInternalServerError, "verify_failed")
		}
		return
	}

	// Payment receipt email is sent by the Razorpay webhook (payment.captured)
	// so it fires exactly once even if the browser never reaches this verify
	// call. See the shop notification listener wired in main.go.

	writeJSON(w, http.StatusOK, res)
}
