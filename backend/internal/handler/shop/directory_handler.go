package shop

import (
	"net/http"

	srv "github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
)

// DirectoryHandler serves the public shop directory (list shops by pincode).
type DirectoryHandler struct {
	svc srv.ShopDirectoryService
}

func NewDirectoryHandler(s srv.ShopDirectoryService) *DirectoryHandler {
	return &DirectoryHandler{svc: s}
}

// List handles GET /api/shop/shops?pincode=<6 digits>
func (h *DirectoryHandler) List(w http.ResponseWriter, r *http.Request) {
	pincode := r.URL.Query().Get("pincode")
	shops, err := h.svc.List(r.Context(), pincode)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "fetch_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"shops": shops})
}
