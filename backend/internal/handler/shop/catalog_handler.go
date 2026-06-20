package shop

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	srv "github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
)

type CatalogHandler struct {
	svc  srv.CatalogService
	feed srv.FeedService
}

func NewCatalogHandler(s srv.CatalogService, f srv.FeedService) *CatalogHandler {
	return &CatalogHandler{s, f}
}

var slugRe = regexp.MustCompile(`^[a-z0-9-]{1,200}$`)

// ── helpers ────────────────────────────────────────────────────────────

func writeJSONWithHeaders(w http.ResponseWriter, status int, v any, cacheControl string, serverTiming string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", cacheControl)
	if serverTiming != "" {
		w.Header().Set("Server-Timing", serverTiming)
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErrShop(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}

// ── handlers ───────────────────────────────────────────────────────────

func (h *CatalogHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	t0 := time.Now()
	cats, err := h.svc.ListCategories(r.Context())
	if err != nil {
		writeErrShop(w, 500, "fetch_failed")
		return
	}
	writeJSONWithHeaders(w, 200, cats,
		"public, max-age=60, stale-while-revalidate=30",
		fmt.Sprintf("db;dur=%.1f", float64(time.Since(t0).Microseconds())/1000.0))
}

func (h *CatalogHandler) ListProducts(w http.ResponseWriter, r *http.Request) {
	t0 := time.Now()
	q := r.URL.Query()
	parsedQ := srv.ProductListQuery{
		CategorySlug: q.Get("category"),
		Search:       q.Get("search"),
		Sort:         q.Get("sort"),
		Cursor:       q.Get("cursor"),
		InStockOnly:  q.Get("in_stock") == "true",
		Limit:        atoiOr(q.Get("limit"), 24),
		Offset:       atoiOr(q.Get("offset"), 0),
	}
	if v := q.Get("price_min"); v != "" {
		n, _ := strconv.ParseInt(v, 10, 64)
		parsedQ.PriceMinPaise = &n
	}
	if v := q.Get("price_max"); v != "" {
		n, _ := strconv.ParseInt(v, 10, 64)
		parsedQ.PriceMaxPaise = &n
	}

	res, err := h.svc.ListProducts(r.Context(), parsedQ)
	if err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "invalid_sort"):
			writeErrShop(w, 400, "invalid_sort")
		case strings.Contains(msg, "invalid price range"):
			writeErrShop(w, 400, "invalid_price_range")
		default:
			writeErrShop(w, 500, "fetch_failed")
		}
		return
	}
	writeJSONWithHeaders(w, 200, res,
		"public, max-age=60, stale-while-revalidate=30",
		fmt.Sprintf("db;dur=%.1f", float64(time.Since(t0).Microseconds())/1000.0))
}

func (h *CatalogHandler) GetProductBySlug(w http.ResponseWriter, r *http.Request) {
	t0 := time.Now()
	slug := chi.URLParam(r, "slug")
	if !slugRe.MatchString(slug) {
		writeErrShop(w, 400, "invalid_slug")
		return
	}

	d, err := h.svc.GetProductBySlug(r.Context(), slug)
	if errors.Is(err, srv.ErrNotFound) || d == nil {
		writeErrShop(w, 404, "not_found")
		return
	}
	if err != nil {
		writeErrShop(w, 500, "fetch_failed")
		return
	}

	body, _ := json.Marshal(d)
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:16]) + `"`
	if r.Header.Get("If-None-Match") == etag {
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "public, max-age=300, stale-while-revalidate=60")
		w.Header().Set("Server-Timing", fmt.Sprintf("db;dur=%.1f", float64(time.Since(t0).Microseconds())/1000.0))
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("ETag", etag)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300, stale-while-revalidate=60")
	w.Header().Set("Server-Timing", fmt.Sprintf("db;dur=%.1f", float64(time.Since(t0).Microseconds())/1000.0))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func atoiOr(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
