package shop

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	srv "github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
)

type OrderHandler struct {
	svc srv.ShopOrderService
}

func NewOrderHandler(s srv.ShopOrderService) *OrderHandler { return &OrderHandler{s} }

func (h *OrderHandler) List(w http.ResponseWriter, r *http.Request) {
	t0 := time.Now()
	customerID, ok := middleware.GetCustomerIDFromContext(r.Context())
	if !ok {
		writeErrShop(w, 401, "unauthorized")
		return
	}

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	res, err := h.svc.List(r.Context(), customerID, srv.OrderListQuery{
		Limit:  limit,
		Cursor: q.Get("cursor"),
	})
	if err != nil {
		writeErrShop(w, 500, "fetch_failed")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Server-Timing", fmt.Sprintf("db;dur=%.1f", float64(time.Since(t0).Microseconds())/1000.0))
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(res)
}

func (h *OrderHandler) Get(w http.ResponseWriter, r *http.Request) {
	t0 := time.Now()
	customerID, ok := middleware.GetCustomerIDFromContext(r.Context())
	if !ok {
		writeErrShop(w, 401, "unauthorized")
		return
	}

	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErrShop(w, 400, "invalid_id")
		return
	}

	d, err := h.svc.Get(r.Context(), customerID, orderID)
	if errors.Is(err, srv.ErrOrderNotFound) {
		writeErrShop(w, 404, "not_found")
		return
	}
	if err != nil {
		writeErrShop(w, 500, "fetch_failed")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Server-Timing", fmt.Sprintf("db;dur=%.1f", float64(time.Since(t0).Microseconds())/1000.0))
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(d)
}

func (h *OrderHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	t0 := time.Now()
	customerID, ok := middleware.GetCustomerIDFromContext(r.Context())
	if !ok {
		writeErrShop(w, 401, "unauthorized")
		return
	}

	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErrShop(w, 400, "invalid_id")
		return
	}

	res, err := h.svc.Cancel(r.Context(), customerID, orderID)
	switch {
	case errors.Is(err, srv.ErrOrderNotFound):
		writeErrShop(w, 404, "not_found")
		return
	case errors.Is(err, srv.ErrCancelNotAllowed):
		writeErrShop(w, 409, "conflict_state")
		return
	case errors.Is(err, srv.ErrRefundFailed):
		writeErrShop(w, 502, "refund_failed")
		return
	case err != nil:
		writeErrShop(w, 500, "fetch_failed")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Server-Timing", fmt.Sprintf("db;dur=%.1f", float64(time.Since(t0).Microseconds())/1000.0))
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(res)
}
