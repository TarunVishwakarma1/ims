package shop

import (
	"encoding/json"
	"errors"
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

// AddItem adds or updates a product in the customer's cart. The shop is taken
// from the route ({shop} → ResolveShop). When the cart already holds items from
// a different shop, responds 409 cart_other_shop with that shop's identity so
// the UI can prompt; ?replace=true clears the old cart and switches shops.
func (h *CartHandler) AddItem(w http.ResponseWriter, r *http.Request) {
	cid, _ := middleware.GetCustomerIDFromContext(r.Context())
	var req addItemReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_body")
		return
	}
	replace := r.URL.Query().Get("replace") == "true"
	v, err := h.svc.AddOrSet(r.Context(), cid, req.ProductID, req.Qty, replace)
	if err != nil {
		var conflict *srv.CartShopConflict
		if errors.As(err, &conflict) {
			writeJSON(w, http.StatusConflict, map[string]any{
				"error": "cart_other_shop",
				"current_shop": map[string]string{
					"slug": conflict.CurrentSlug,
					"name": conflict.CurrentName,
				},
			})
			return
		}
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
