package shop

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	srv "github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
)

// AdminOrderHandler is the back-office surface for B2C shop orders: list and
// advance status (confirm → ship → deliver). Mounted behind admin auth +
// banners/orders RBAC, separate from the customer-facing OrderHandler.
type AdminOrderHandler struct {
	svc      srv.ShopOrderService
	notifier *srv.ShopNotifier // may be nil — status emails disabled
}

func NewAdminOrderHandler(s srv.ShopOrderService, notifier *srv.ShopNotifier) *AdminOrderHandler {
	return &AdminOrderHandler{svc: s, notifier: notifier}
}

// List handles GET /api/admin/shop/orders?status=&limit=&offset=
func (h *AdminOrderHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	orders, err := h.svc.AdminList(r.Context(), q.Get("status"), limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "fetch_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": orders})
}

type advanceStatusReq struct {
	Status string `json:"status"`
}

// UpdateStatus handles PUT /api/admin/shop/orders/{id}/status
func (h *AdminOrderHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_id")
		return
	}
	var req advanceStatusReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_body")
		return
	}

	customerID, err := h.svc.AdvanceStatus(r.Context(), orderID, req.Status)
	if err != nil {
		switch {
		case errors.Is(err, srv.ErrOrderNotFound):
			writeErr(w, http.StatusNotFound, "not_found")
		case errors.Is(err, srv.ErrInvalidTransition):
			writeErr(w, http.StatusConflict, "invalid_transition")
		default:
			writeErr(w, http.StatusInternalServerError, "update_failed")
		}
		return
	}

	// Notify the customer on the shipping milestones.
	if req.Status == "shipped" || req.Status == "delivered" {
		h.notifier.OrderStatusChanged(r.Context(), customerID, orderID, req.Status)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": req.Status})
}
