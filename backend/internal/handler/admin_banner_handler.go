package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	srv "github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/pkg/storage"
)

type AdminBannerHandler struct {
	svc      srv.BannerService
	store    storage.Storage
	maxBytes int64
}

func NewAdminBannerHandler(s srv.BannerService, store storage.Storage, maxBytes int64) *AdminBannerHandler {
	return &AdminBannerHandler{svc: s, store: store, maxBytes: maxBytes}
}

func (h *AdminBannerHandler) Upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.maxBytes+1024) // small overhead allowance
	if err := r.ParseMultipartForm(h.maxBytes + 1024); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_image")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_image")
		return
	}
	defer file.Close()
	if header.Size > h.maxBytes {
		writeError(w, http.StatusBadRequest, "invalid_image")
		return
	}

	// MIME sniff first 512 bytes.
	head := make([]byte, 512)
	n, _ := file.Read(head)
	ct := http.DetectContentType(head[:n])
	ext := mimeExt(ct)
	if ext == "" {
		writeError(w, http.StatusBadRequest, "invalid_image")
		return
	}

	// Rewind to start.
	if _, err := file.Seek(0, 0); err != nil {
		writeError(w, http.StatusInternalServerError, "upload_failed")
		return
	}

	key := "banners/" + uuid.New().String() + ext
	url, err := h.store.Save(r.Context(), key, file, ct)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "upload_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"image_url": url})
}

func mimeExt(ct string) string {
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

func (h *AdminBannerHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	res, err := h.svc.List(r.Context(), srv.BannerListQuery{
		Status:   q.Get("status"),
		EventKey: q.Get("event_key"),
		Limit:    atoiOrDef(q.Get("limit"), 24),
		Offset:   atoiOrDef(q.Get("offset"), 0),
	})
	if err != nil {
		mapBannerErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *AdminBannerHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	b, err := h.svc.Get(r.Context(), id)
	if err != nil {
		mapBannerErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (h *AdminBannerHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in srv.BannerInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	out, err := h.svc.Create(r.Context(), in)
	if err != nil {
		mapBannerErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *AdminBannerHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	var in srv.BannerInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	out, err := h.svc.Update(r.Context(), id, in)
	if err != nil {
		mapBannerErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *AdminBannerHandler) Publish(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	if err := h.svc.Publish(r.Context(), id); err != nil {
		mapBannerErr(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *AdminBannerHandler) Archive(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	if err := h.svc.Archive(r.Context(), id); err != nil {
		mapBannerErr(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *AdminBannerHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		mapBannerErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func mapBannerErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, srv.ErrBannerNotFound):
		writeError(w, http.StatusNotFound, "not_found")
	case errors.Is(err, srv.ErrHeroConflict):
		writeError(w, http.StatusConflict, "hero_conflict")
	case errors.Is(err, srv.ErrInvalidStatus):
		writeError(w, http.StatusBadRequest, "invalid_status")
	case errors.Is(err, srv.ErrInvalidDateRange):
		writeError(w, http.StatusBadRequest, "invalid_dates")
	case errors.Is(err, srv.ErrImageRequired):
		writeError(w, http.StatusBadRequest, "image_required")
	case errors.Is(err, srv.ErrInvalidAudience):
		writeError(w, http.StatusBadRequest, "invalid_audience")
	default:
		writeError(w, http.StatusInternalServerError, "fetch_failed")
	}
}

// atoiOrDef parses a decimal string s and returns its integer value,
// or def if s is empty or contains non-digit characters.
func atoiOrDef(s string, def int) int {
	if s == "" {
		return def
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return def
		}
		n = n*10 + int(r-'0')
	}
	return n
}
