package handler

import (
	"errors"
	"net/http"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type NotificationHandler struct {
	service service.NotificationService
}

func NewNotificationHandler(s service.NotificationService) *NotificationHandler {
	return &NotificationHandler{service: s}
}

// List — GET /api/notifications?status=pending|sent|failed|dlq
func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	status := domain.NotificationStatus(r.URL.Query().Get("status"))
	if status == "" {
		status = domain.NotificationStatusDLQ
	}
	list, err := h.service.ListByStatus(r.Context(), status)
	if err != nil {
		zap.L().Error("ListNotifications failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if list == nil {
		list = []*domain.Notification{}
	}
	writeJSON(w, http.StatusOK, list)
}

// GetByID — GET /api/notifications/{id}
func (h *NotificationHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	n, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "notification not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, n)
}

// Resend — POST /api/notifications/{id}/resend
func (h *NotificationHandler) Resend(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.service.Resend(r.Context(), id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "notification not found")
			return
		}
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "queued for resend"})
}
