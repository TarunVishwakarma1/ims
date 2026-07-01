package shop

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	srv "github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
)

// ProductStorefrontHandler manages a product's storefront overlay (visibility,
// shop slug/price/description/images), org-scoped.
type ProductStorefrontHandler struct {
	svc srv.ProductStorefrontService
}

func NewProductStorefrontHandler(s srv.ProductStorefrontService) *ProductStorefrontHandler {
	return &ProductStorefrontHandler{svc: s}
}

type productStorefrontRequest struct {
	ShopVisible     bool     `json:"shop_visible"`
	ShopSlug        string   `json:"shop_slug"`
	ShopPricePaise  *int64   `json:"shop_price_paise"`
	ShopDescription string   `json:"shop_description"`
	ShopImageURLs   []string `json:"shop_image_urls"`
}

func (h *ProductStorefrontHandler) ctx(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return uuid.Nil, uuid.Nil, false
	}
	productID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_id")
		return uuid.Nil, uuid.Nil, false
	}
	return orgID, productID, true
}

func (h *ProductStorefrontHandler) Get(w http.ResponseWriter, r *http.Request) {
	orgID, productID, ok := h.ctx(w, r)
	if !ok {
		return
	}
	ps, err := h.svc.Get(r.Context(), orgID, productID)
	if errors.Is(err, domain.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, ps)
}

func (h *ProductStorefrontHandler) Set(w http.ResponseWriter, r *http.Request) {
	orgID, productID, ok := h.ctx(w, r)
	if !ok {
		return
	}
	var req productStorefrontRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request")
		return
	}
	ps, err := h.svc.Set(r.Context(), orgID, srv.ProductStorefront{
		ProductID: productID, ShopVisible: req.ShopVisible, ShopSlug: req.ShopSlug,
		ShopPricePaise: req.ShopPricePaise, ShopDescription: req.ShopDescription,
		ShopImageURLs: req.ShopImageURLs,
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeErr(w, http.StatusNotFound, "not_found")
		case errors.Is(err, srv.ErrShopSlugTaken):
			writeErr(w, http.StatusConflict, err.Error())
		case errors.Is(err, srv.ErrShopSlugRequired), errors.Is(err, srv.ErrInvalidProfileSlug):
			writeErr(w, http.StatusUnprocessableEntity, err.Error())
		default:
			writeErr(w, http.StatusInternalServerError, "internal")
		}
		return
	}
	writeJSON(w, http.StatusOK, ps)
}
