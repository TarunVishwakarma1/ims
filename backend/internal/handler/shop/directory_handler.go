package shop

import (
	"net/http"
	"strconv"

	srv "github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
)

// DirectoryHandler serves the public shop directory (list shops by pincode).
type DirectoryHandler struct {
	svc srv.ShopDirectoryService
}

func NewDirectoryHandler(s srv.ShopDirectoryService) *DirectoryHandler {
	return &DirectoryHandler{svc: s}
}

// List handles GET /api/shop/shops. With ?lat=&lng= it returns shops whose
// delivery radius covers that point (nearest first); otherwise it lists shops,
// optionally filtered by ?pincode=<6 digits>.
func (h *DirectoryHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	latStr, lngStr := q.Get("lat"), q.Get("lng")
	if latStr != "" && lngStr != "" {
		lat, errLat := strconv.ParseFloat(latStr, 64)
		lng, errLng := strconv.ParseFloat(lngStr, 64)
		if errLat != nil || errLng != nil ||
			lat < -90 || lat > 90 || lng < -180 || lng > 180 {
			writeErr(w, http.StatusBadRequest, "invalid_location")
			return
		}
		shops, err := h.svc.ListNearby(r.Context(), lat, lng)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "fetch_failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"shops": shops})
		return
	}

	shops, err := h.svc.List(r.Context(), q.Get("pincode"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "fetch_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"shops": shops})
}
