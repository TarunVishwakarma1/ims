package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/service"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type CouponHandler struct {
	service service.CouponService
}

func NewCouponHandler(s service.CouponService) *CouponHandler {
	return &CouponHandler{service: s}
}

// List — GET /api/coupons
func (h *CouponHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing org context")
		return
	}
	list, err := h.service.List(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list coupons")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// GetByID — GET /api/coupons/{id}
func (h *CouponHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing org context")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid coupon id")
		return
	}
	c, err := h.service.GetByID(r.Context(), id, orgID)
	if err != nil {
		writeError(w, http.StatusNotFound, "coupon not found")
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// Create — POST /api/coupons
func (h *CouponHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing org context")
		return
	}
	var c domain.Coupon
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.service.Create(r.Context(), &c, orgID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

// Update — PUT /api/coupons/{id}
func (h *CouponHandler) Update(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing org context")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid coupon id")
		return
	}
	var c domain.Coupon
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	c.ID = id
	if err := h.service.Update(r.Context(), &c, orgID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// Delete — DELETE /api/coupons/{id}
func (h *CouponHandler) Delete(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing org context")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid coupon id")
		return
	}
	if err := h.service.Delete(r.Context(), id, orgID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type validateReq struct {
	SupplierOrgID string `json:"supplier_org_id"`
	Code          string `json:"code"`
	Subtotal      int64  `json:"subtotal"` // paise
}

// Validate — POST /api/coupons/validate
// Stateless preview: returns the discount amount without applying it.
// Used by the cart to show the resulting total before checkout.
func (h *CouponHandler) Validate(w http.ResponseWriter, r *http.Request) {
	var req validateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	supplierID, err := uuid.Parse(strings.TrimSpace(req.SupplierOrgID))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid supplier_org_id")
		return
	}
	c, amountOff, err := h.service.Validate(r.Context(), supplierID, req.Code, req.Subtotal)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"coupon":     c,
		"amount_off": amountOff,
	})
}
