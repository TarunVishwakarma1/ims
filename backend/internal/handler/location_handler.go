package handler

import (
	"encoding/json"
	"net/http"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/service"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type LocationHandler struct {
	service service.LocationService
}

func NewLocationHandler(s service.LocationService) *LocationHandler {
	return &LocationHandler{service: s}
}

func (h *LocationHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing org context")
		return
	}

	locations, err := h.service.List(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if locations == nil {
		locations = []*domain.OrgLocation{}
	}
	writeJSON(w, http.StatusOK, locations)
}

func (h *LocationHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing org context")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req domain.OrgLocation
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.OrgID = orgID

	if err := h.service.Create(r.Context(), &req); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, req)
}

func (h *LocationHandler) Update(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing org context")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid location id")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req domain.OrgLocation
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.ID = id
	req.OrgID = orgID

	if err := h.service.Update(r.Context(), &req); err != nil {
		if err == domain.ErrNotFound {
			writeError(w, http.StatusNotFound, "location not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, req)
}

func (h *LocationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing org context")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid location id")
		return
	}

	if err := h.service.Delete(r.Context(), id, orgID); err != nil {
		if err == domain.ErrNotFound {
			writeError(w, http.StatusNotFound, "location not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "location deleted"})
}
