package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/service"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type ReturnHandler struct {
	service service.ReturnService
}

func NewReturnHandler(s service.ReturnService) *ReturnHandler {
	return &ReturnHandler{service: s}
}

type createReturnReq struct {
	Reason string `json:"reason"`
	Items  []struct {
		OrderItemID uuid.UUID `json:"order_item_id"`
		Quantity    int       `json:"quantity"`
	} `json:"items"`
}

// Create — POST /api/orders/{id}/returns
func (h *ReturnHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing identity")
		return
	}
	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid order id")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req createReturnReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	items := make([]service.CreateReturnItem, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, service.CreateReturnItem{OrderItemID: it.OrderItemID, Quantity: it.Quantity})
	}

	rr, err := h.service.Create(r.Context(), actor.OrgID, actor.UserID, service.CreateReturnInput{
		OrderID: orderID,
		Reason:  req.Reason,
		Items:   items,
	})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "order not found")
			return
		}
		// Business-rule errors (eligibility, quantity, missing item) → 400
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, rr)
}

// ListByOrder — GET /api/orders/{id}/returns
func (h *ReturnHandler) ListByOrder(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing identity")
		return
	}
	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid order id")
		return
	}
	list, err := h.service.ListByOrder(r.Context(), orderID, actor.OrgID)
	if err != nil {
		zap.L().Error("ListReturnsByOrder failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// ListAll — GET /api/returns
func (h *ReturnHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing identity")
		return
	}
	list, err := h.service.ListByOrg(r.Context(), actor.OrgID)
	if err != nil {
		zap.L().Error("ListReturns failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// GetByID — GET /api/returns/{id}
func (h *ReturnHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing identity")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid return id")
		return
	}
	rr, err := h.service.GetByID(r.Context(), id, actor.OrgID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "return not found")
			return
		}
		zap.L().Error("GetReturn failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, rr)
}

// Approve — POST /api/returns/{id}/approve
func (h *ReturnHandler) Approve(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, h.service.Approve)
}

// Reject — POST /api/returns/{id}/reject  body: { note: string }
func (h *ReturnHandler) Reject(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing identity")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid return id")
		return
	}
	var body struct {
		Note string `json:"note"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := h.service.Reject(r.Context(), id, actor.OrgID, body.Note); err != nil {
		writeReturnErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "return rejected"})
}

// MarkReceived — POST /api/returns/{id}/received
func (h *ReturnHandler) MarkReceived(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, h.service.MarkReceived)
}

// transition factors out the shared id-parsing + error mapping for
// Approve/MarkReceived (Reject differs because it takes a note in the body).
func (h *ReturnHandler) transition(w http.ResponseWriter, r *http.Request, fn func(ctx context.Context, id, orgID uuid.UUID) error) {
	actor, ok := middleware.ActorFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing identity")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid return id")
		return
	}
	if err := fn(r.Context(), id, actor.OrgID); err != nil {
		writeReturnErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "ok"})
}

func writeReturnErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "return not found")
	case errors.Is(err, domain.ErrConflict):
		writeError(w, http.StatusConflict, "illegal status transition")
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}
