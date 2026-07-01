package shop

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	shopsvc "github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
	"github.com/TarunVishwakarma1/ims/backend/pkg/storage"
)

type ProfileHandler struct {
	svc      shopsvc.ShopProfileService
	store    storage.Storage
	maxBytes int64
}

func NewProfileHandler(svc shopsvc.ShopProfileService, store storage.Storage, maxBytes int64) *ProfileHandler {
	return &ProfileHandler{svc: svc, store: store, maxBytes: maxBytes}
}

type profileRequest struct {
	Slug        string   `json:"slug"`
	DisplayName string   `json:"display_name"`
	Tagline     string   `json:"tagline"`
	LogoURL     string   `json:"logo_url"`
	Area        string   `json:"area"`
	City        string   `json:"city"`
	Pincodes         []string `json:"pincodes"`
	Lat              *float64 `json:"lat"`
	Lng              *float64 `json:"lng"`
	DeliveryRadiusKm *float64 `json:"delivery_radius_km"`
	IsLive           bool     `json:"is_live"`
}

func (h *ProfileHandler) GetMine(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	p, err := h.svc.GetMine(r.Context(), orgID)
	if errors.Is(err, domain.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "no_profile")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *ProfileHandler) Upsert(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req profileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request")
		return
	}
	p, err := h.svc.Upsert(r.Context(), orgID, shopsvc.UpsertProfileInput{
		Slug: req.Slug, DisplayName: req.DisplayName, Tagline: req.Tagline,
		LogoURL: req.LogoURL, Area: req.Area, City: req.City,
		Pincodes: req.Pincodes, Lat: req.Lat, Lng: req.Lng,
		DeliveryRadiusKm: req.DeliveryRadiusKm, IsLive: req.IsLive,
	})
	if err != nil {
		switch {
		case errors.Is(err, shopsvc.ErrSlugTaken), errors.Is(err, shopsvc.ErrSlugLocked):
			writeErr(w, http.StatusConflict, err.Error())
		case errors.Is(err, shopsvc.ErrInvalidProfileSlug), errors.Is(err, shopsvc.ErrGoLiveIncomplete):
			writeErr(w, http.StatusUnprocessableEntity, err.Error())
		default:
			writeErr(w, http.StatusInternalServerError, "internal")
		}
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// LogoUpload accepts a multipart "file" image, stores it under storefront/, and
// returns its public URL for the caller to save into logo_url. Mirrors the
// admin banner upload: MIME-sniffed, size-capped, only jpeg/png/webp.
func (h *ProfileHandler) LogoUpload(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeErr(w, http.StatusNotImplemented, "uploads_disabled")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, h.maxBytes+1024)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_image")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_image")
		return
	}
	defer file.Close()

	// Sniff the content type from the first 512 bytes, then rewind.
	head := make([]byte, 512)
	n, _ := file.Read(head)
	ct := http.DetectContentType(head[:n])
	ext := imageExt(ct)
	if ext == "" {
		writeErr(w, http.StatusBadRequest, "invalid_image")
		return
	}
	if _, err := file.Seek(0, 0); err != nil {
		writeErr(w, http.StatusInternalServerError, "upload_failed")
		return
	}

	key := "storefront/" + uuid.New().String() + ext
	url, err := h.store.Save(r.Context(), key, file, ct)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "upload_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"logo_url": url})
}

func imageExt(ct string) string {
	switch ct {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	}
	return ""
}
