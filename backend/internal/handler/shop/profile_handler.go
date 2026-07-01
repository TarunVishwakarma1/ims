package shop

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	shopsvc "github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
)

type ProfileHandler struct{ svc shopsvc.ShopProfileService }

func NewProfileHandler(svc shopsvc.ShopProfileService) *ProfileHandler {
	return &ProfileHandler{svc: svc}
}

type profileRequest struct {
	Slug        string   `json:"slug"`
	DisplayName string   `json:"display_name"`
	Tagline     string   `json:"tagline"`
	LogoURL     string   `json:"logo_url"`
	Area        string   `json:"area"`
	City        string   `json:"city"`
	Pincodes    []string `json:"pincodes"`
	Lat         *float64 `json:"lat"`
	Lng         *float64 `json:"lng"`
	IsLive      bool     `json:"is_live"`
}

func (h *ProfileHandler) GetMine(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	p, err := h.svc.GetMine(r.Context(), orgID)
	if errors.Is(err, domain.ErrNotFound) {
		http.Error(w, `{"error":"no_profile"}`, http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *ProfileHandler) Upsert(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	var req profileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad_request"}`, http.StatusBadRequest)
		return
	}
	p, err := h.svc.Upsert(r.Context(), orgID, shopsvc.UpsertProfileInput{
		Slug: req.Slug, DisplayName: req.DisplayName, Tagline: req.Tagline,
		LogoURL: req.LogoURL, Area: req.Area, City: req.City,
		Pincodes: req.Pincodes, Lat: req.Lat, Lng: req.Lng, IsLive: req.IsLive,
	})
	if err != nil {
		switch {
		case errors.Is(err, shopsvc.ErrSlugTaken), errors.Is(err, shopsvc.ErrSlugLocked):
			writeErr(w, http.StatusConflict, err.Error())
		case errors.Is(err, shopsvc.ErrInvalidProfileSlug), errors.Is(err, shopsvc.ErrGoLiveIncomplete):
			writeErr(w, http.StatusUnprocessableEntity, err.Error())
		default:
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		}
		return
	}
	writeJSON(w, http.StatusOK, p)
}
