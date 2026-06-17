package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type AuditHandler struct {
	repo repository.AuditLogRepository
}

func NewAuditHandler(repo repository.AuditLogRepository) *AuditHandler {
	return &AuditHandler{repo: repo}
}

// List — GET /api/audit?entity=&action_prefix=&user_id=&from=&to=&page=&per_page=
func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing identity")
		return
	}
	q := r.URL.Query()
	f := repository.AuditFilters{
		Entity:       q.Get("entity"),
		ActionPrefix: q.Get("action_prefix"),
	}
	if v := q.Get("user_id"); v != "" {
		uid, err := uuid.Parse(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid user_id")
			return
		}
		f.UserID = &uid
	}
	if v := q.Get("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "from must be RFC3339")
			return
		}
		f.From = &t
	}
	if v := q.Get("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "to must be RFC3339")
			return
		}
		f.To = &t
	}
	if v := q.Get("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "invalid page")
			return
		}
		f.Page = n
	}
	if v := q.Get("per_page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "invalid per_page")
			return
		}
		f.PerPage = n
	}

	items, total, err := h.repo.ListFiltered(r.Context(), actor.OrgID, f)
	if err != nil {
		zap.L().Error("audit list failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if items == nil {
		items = nil // explicit
	}
	if errors.Is(err, nil) {
		writeJSON(w, http.StatusOK, map[string]any{
			"items":    items,
			"total":    total,
			"page":     max1(f.Page),
			"per_page": defOr(f.PerPage, 50),
		})
	}
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

func defOr(n, def int) int {
	if n <= 0 {
		return def
	}
	return n
}
