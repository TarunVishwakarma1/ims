package shop

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	srv "github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
)

type CartHandler struct {
	svc srv.CartService
}

func NewCartHandler(s srv.CartService) *CartHandler {
	return &CartHandler{svc: s}
}

type addItemReq struct {
	ProductID uuid.UUID `json:"product_id"`
	Qty       int       `json:"qty"`
}

// Get returns the current cart for the customer
func (h *CartHandler) Get(w http.ResponseWriter, r *http.Request) {
	cid, _ := middleware.GetCustomerIDFromContext(r.Context())
	v, err := h.svc.Get(r.Context(), cid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "fetch_failed")
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// AddItem adds or updates a product in the customer's cart
func (h *CartHandler) AddItem(w http.ResponseWriter, r *http.Request) {
	cid, _ := middleware.GetCustomerIDFromContext(r.Context())
	var req addItemReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_body")
		return
	}
	v, err := h.svc.AddOrSet(r.Context(), cid, req.ProductID, req.Qty)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// RemoveItem removes a product from the customer's cart
func (h *CartHandler) RemoveItem(w http.ResponseWriter, r *http.Request) {
	cid, _ := middleware.GetCustomerIDFromContext(r.Context())
	pid, err := uuid.Parse(chi.URLParam(r, "product_id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_id")
		return
	}
	v, err := h.svc.Remove(r.Context(), cid, pid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "remove_failed")
		return
	}
	writeJSON(w, http.StatusOK, v)
}

type mergeReq struct {
	Items []struct {
		ProductID uuid.UUID `json:"product_id"`
		Qty       int       `json:"qty"`
	} `json:"items"`
}

// Merge merges a list of items into the customer's cart
func (h *CartHandler) Merge(w http.ResponseWriter, r *http.Request) {
	cid, _ := middleware.GetCustomerIDFromContext(r.Context())
	var req mergeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_body")
		return
	}
	items := make([]srv.MergeItem, len(req.Items))
	for i, it := range req.Items {
		items[i] = srv.MergeItem{ProductID: it.ProductID, Qty: it.Qty}
	}
	v, err := h.svc.Merge(r.Context(), cid, items)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "merge_failed")
		return
	}
	writeJSON(w, http.StatusOK, v)
}
