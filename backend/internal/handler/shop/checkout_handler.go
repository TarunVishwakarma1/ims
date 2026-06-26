package shop

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	srv "github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
)

// CheckoutHandler handles GET /checkout/summary and POST /checkout/place.
type CheckoutHandler struct {
	svc srv.CheckoutService
}

// NewCheckoutHandler constructs a CheckoutHandler backed by the given service.
func NewCheckoutHandler(s srv.CheckoutService) *CheckoutHandler {
	return &CheckoutHandler{s}
}

// Summary handles GET /checkout/summary?address_id=<uuid>.
func (h *CheckoutHandler) Summary(w http.ResponseWriter, r *http.Request) {
	cid, _ := middleware.GetCustomerIDFromContext(r.Context())

	addrID, err := uuid.Parse(r.URL.Query().Get("address_id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "address_required")
		return
	}

	s, err := h.svc.Summary(r.Context(), cid, addrID)
	if err != nil {
		switch {
		case errors.Is(err, srv.ErrCartEmpty):
			writeErr(w, http.StatusConflict, "cart_empty")
		default:
			writeErr(w, http.StatusInternalServerError, "summary_failed")
		}
		return
	}

	writeJSON(w, http.StatusOK, s)
}

// PaymentOptions handles GET /api/shop/checkout/payment-options.
func (h *CheckoutHandler) PaymentOptions(w http.ResponseWriter, r *http.Request) {
	cid, _ := middleware.GetCustomerIDFromContext(r.Context())
	opts, err := h.svc.PaymentOptions(r.Context(), cid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "options_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"methods": opts})
}

// placeReq is the JSON body for POST /checkout/place.
type placeReq struct {
	AddressID     uuid.UUID `json:"address_id"`
	PaymentMethod string    `json:"payment_method"`
	Notes         string    `json:"notes"`
}

// Place handles POST /checkout/place.
func (h *CheckoutHandler) Place(w http.ResponseWriter, r *http.Request) {
	cid, _ := middleware.GetCustomerIDFromContext(r.Context())

	var req placeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_body")
		return
	}

	res, err := h.svc.Place(r.Context(), srv.PlaceOrderInput{
		CustomerID:    cid,
		AddressID:     req.AddressID,
		PaymentMethod: req.PaymentMethod,
		Notes:         req.Notes,
	})
	if err != nil {
		switch {
		case errors.Is(err, srv.ErrCartEmpty):
			writeErr(w, http.StatusConflict, "cart_empty")
		case errors.Is(err, srv.ErrAddressRequired):
			writeErr(w, http.StatusBadRequest, "address_required")
		case errors.Is(err, srv.ErrInsufficientStock):
			writeErr(w, http.StatusConflict, "stock_unavailable")
		case errors.Is(err, srv.ErrInvalidPaymentMethod):
			writeErr(w, http.StatusBadRequest, "invalid_payment_method")
		case errors.Is(err, srv.ErrCODIneligible):
			writeErr(w, http.StatusBadRequest, "cod_ineligible")
		default:
			writeErr(w, http.StatusInternalServerError, "place_failed")
		}
		return
	}

	writeJSON(w, http.StatusOK, res)
}
