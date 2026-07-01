package shop

import (
	"net/http"
	"strconv"

	srv "github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
)

// AnalyticsHandler serves a shop's own sales analytics (org-scoped).
type AnalyticsHandler struct {
	svc srv.ShopAnalyticsService
}

func NewAnalyticsHandler(s srv.ShopAnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{svc: s}
}

// Sales handles GET /api/admin/shop/analytics?days=<1-365> (default 30).
func (h *AnalyticsHandler) Sales(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil {
			days = n
		}
	}
	sum, err := h.svc.SalesSummary(r.Context(), orgID, days)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "fetch_failed")
		return
	}
	writeJSON(w, http.StatusOK, sum)
}
