package shop

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	srv "github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
)

type BannerHandler struct {
	svc srv.BannerService
}

func NewBannerHandler(s srv.BannerService) *BannerHandler { return &BannerHandler{s} }

func (h *BannerHandler) ListActive(w http.ResponseWriter, r *http.Request) {
	t0 := time.Now()
	cat := r.URL.Query().Get("category")
	pg, err := h.svc.ListActive(r.Context(), cat)
	if err != nil {
		writeErrShop(w, 500, "fetch_failed")
		return
	}
	body, _ := json.Marshal(pg)
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:16]) + `"`
	timing := fmt.Sprintf("db;dur=%.1f", float64(time.Since(t0).Microseconds())/1000.0)
	if r.Header.Get("If-None-Match") == etag {
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "public, max-age=60, stale-while-revalidate=30")
		w.Header().Set("Server-Timing", timing)
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("ETag", etag)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=60, stale-while-revalidate=30")
	w.Header().Set("Server-Timing", timing)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
