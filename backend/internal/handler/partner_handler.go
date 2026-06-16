package handler

import (
	"errors"
	"net/http"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/service"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type PartnerHandler struct {
	service service.PartnerService
}

func NewPartnerHandler(s service.PartnerService) *PartnerHandler {
	return &PartnerHandler{service: s}
}

// List handles GET /api/partners?role=supplier|buyer
func (h *PartnerHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing org context")
		return
	}
	role := r.URL.Query().Get("role")
	if role != "supplier" && role != "buyer" {
		writeError(w, http.StatusBadRequest, "role must be 'supplier' or 'buyer'")
		return
	}
	partners, err := h.service.List(r.Context(), orgID, role)
	if err != nil {
		zap.L().Error("ListPartners failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if partners == nil {
		partners = []*domain.Partner{}
	}
	writeJSON(w, http.StatusOK, partners)
}

// Detail handles GET /api/partners/{id}
func (h *PartnerHandler) Detail(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing org context")
		return
	}
	partnerID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid partner id")
		return
	}
	detail, err := h.service.Detail(r.Context(), orgID, partnerID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "partner not found")
			return
		}
		zap.L().Error("PartnerDetail failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, detail)
}
