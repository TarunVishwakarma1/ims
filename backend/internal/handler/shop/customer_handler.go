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

// CustomerHandler handles HTTP requests for customer profile and addresses.
type CustomerHandler struct {
	svc srv.CustomerService
}

// NewCustomerHandler creates a CustomerHandler wired to the given service.
func NewCustomerHandler(s srv.CustomerService) *CustomerHandler {
	return &CustomerHandler{svc: s}
}

// GetMe handles GET /me — returns the authenticated customer's profile.
func (h *CustomerHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	cid, _ := middleware.GetCustomerIDFromContext(r.Context())
	c, err := h.svc.Get(r.Context(), cid)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "not_found")
			return
		}
		writeErr(w, http.StatusInternalServerError, "fetch_failed")
		return
	}
	writeJSON(w, http.StatusOK, c)
}

type updateMeBody struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// UpdateMe handles PATCH /me — updates the authenticated customer's profile.
func (h *CustomerHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	cid, _ := middleware.GetCustomerIDFromContext(r.Context())
	var req updateMeBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if err := h.svc.Update(r.Context(), cid, req.Name, req.Email); err != nil {
		writeErr(w, http.StatusInternalServerError, "update_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ListAddresses handles GET /addresses — lists all addresses for the customer.
func (h *CustomerHandler) ListAddresses(w http.ResponseWriter, r *http.Request) {
	cid, _ := middleware.GetCustomerIDFromContext(r.Context())
	addrs, err := h.svc.ListAddresses(r.Context(), cid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "fetch_failed")
		return
	}
	// Encode a missing list as [] rather than null so JSON consumers can
	// safely call .map without a guard.
	if addrs == nil {
		addrs = []domain.CustomerAddress{}
	}
	writeJSON(w, http.StatusOK, addrs)
}

type addrBody struct {
	Label      string   `json:"label"`
	Line1      string   `json:"line1"`
	Line2      string   `json:"line2"`
	City       string   `json:"city"`
	State      string   `json:"state"`
	PostalCode string   `json:"postal_code"`
	Lat        *float64 `json:"lat"`
	Lng        *float64 `json:"lng"`
	IsDefault  bool     `json:"is_default"`
}

// AddAddress handles POST /addresses — adds a new address for the customer.
func (h *CustomerHandler) AddAddress(w http.ResponseWriter, r *http.Request) {
	cid, _ := middleware.GetCustomerIDFromContext(r.Context())
	var req addrBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if req.Line1 == "" || req.City == "" || req.State == "" || req.PostalCode == "" {
		writeErr(w, http.StatusBadRequest, "missing_fields")
		return
	}
	a := &domain.CustomerAddress{
		Label:      req.Label,
		Line1:      req.Line1,
		Line2:      req.Line2,
		City:       req.City,
		State:      req.State,
		PostalCode: req.PostalCode,
		Lat:        req.Lat,
		Lng:        req.Lng,
		IsDefault:  req.IsDefault,
	}
	id, err := h.svc.AddAddress(r.Context(), cid, a)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "create_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]uuid.UUID{"id": id})
}

// UpdateAddress handles PATCH /addresses/{id} — updates an existing address.
func (h *CustomerHandler) UpdateAddress(w http.ResponseWriter, r *http.Request) {
	cid, _ := middleware.GetCustomerIDFromContext(r.Context())
	addrID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_id")
		return
	}
	var req addrBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_body")
		return
	}
	a := &domain.CustomerAddress{
		ID:         addrID,
		Label:      req.Label,
		Line1:      req.Line1,
		Line2:      req.Line2,
		City:       req.City,
		State:      req.State,
		PostalCode: req.PostalCode,
		Lat:        req.Lat,
		Lng:        req.Lng,
		IsDefault:  req.IsDefault,
	}
	if err := h.svc.UpdateAddress(r.Context(), cid, a); err != nil {
		if err.Error() == "forbidden" {
			writeErr(w, http.StatusForbidden, "forbidden")
			return
		}
		writeErr(w, http.StatusInternalServerError, "update_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DeleteAddress handles DELETE /addresses/{id} — removes an address.
func (h *CustomerHandler) DeleteAddress(w http.ResponseWriter, r *http.Request) {
	cid, _ := middleware.GetCustomerIDFromContext(r.Context())
	addrID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_id")
		return
	}
	if err := h.svc.DeleteAddress(r.Context(), cid, addrID); err != nil {
		writeErr(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SetDefaultAddress handles POST /addresses/{id}/default — marks an address as default.
func (h *CustomerHandler) SetDefaultAddress(w http.ResponseWriter, r *http.Request) {
	cid, _ := middleware.GetCustomerIDFromContext(r.Context())
	addrID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_id")
		return
	}
	if err := h.svc.SetDefaultAddress(r.Context(), cid, addrID); err != nil {
		writeErr(w, http.StatusInternalServerError, "update_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
