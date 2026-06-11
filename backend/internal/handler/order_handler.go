package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/service"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
	"github.com/TarunVishwakarma1/ims/backend/pkg/utils"
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type OrderHandler struct {
	service        service.OrderService
	productService service.ProductService
	validate       *validator.Validate
}

func NewOrderHandler(service service.OrderService, productService service.ProductService) *OrderHandler {
	return &OrderHandler{
		service:        service,
		productService: productService,
		validate:       validator.New(),
	}
}

type CreateOrderItemRequest struct {
	ProductID uuid.UUID `json:"product_id" validate:"required"`
	Quantity  int       `json:"quantity" validate:"min=1"`
}

type CreateOrderRequest struct {
	Items []*CreateOrderItemRequest `json:"items" validate:"required,min=1,dive"`
}

type UpdateOrderStatusRequest struct {
	Status domain.OrderStatus `json:"status" validate:"required,oneof=pending confirmed cancelled"`
}

func (h *OrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	userIDStr, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid user ID claim")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB
	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ipAddress := utils.GetClientIP(r)

	order := &domain.Order{
		UserID: userID,
		Status: domain.OrderStatusPending,
	}

	var items []*domain.OrderItem
	for _, item := range req.Items {
		prod, err := h.productService.GetByID(r.Context(), item.ProductID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				writeError(w, http.StatusBadRequest, "product not found: "+item.ProductID.String())
				return
			}
			zap.L().Error("GetByID for product failed", zap.String("product_id", item.ProductID.String()), zap.Error(err))
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		items = append(items, &domain.OrderItem{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
			UnitPrice: prod.Price,
		})
	}

	if err := h.service.Create(r.Context(), order, items, ipAddress); err != nil {
		if errors.Is(err, domain.ErrInsufficientStock) {
			writeError(w, http.StatusBadRequest, "insufficient stock for one or more items")
			return
		}
		zap.L().Error("CreateOrder failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	response := map[string]any{
		"order": order,
		"items": items,
	}
	writeJSON(w, http.StatusCreated, response)
}

func (h *OrderHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid order ID")
		return
	}

	order, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "order not found")
			return
		}
		zap.L().Error("GetOrder failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, order)
}

func (h *OrderHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid order ID")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB
	var req UpdateOrderStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ipAddress := utils.GetClientIP(r)

	if err := h.service.UpdateStatus(r.Context(), id, req.Status, ipAddress); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "order not found")
			return
		}
		zap.L().Error("UpdateStatus failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "order status updated successfully"})
}

func (h *OrderHandler) DeleteOrder(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid order ID")
		return
	}

	ipAddress := utils.GetClientIP(r)

	if err := h.service.Delete(r.Context(), id, ipAddress); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "order not found")
			return
		}
		zap.L().Error("DeleteOrder failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "order deleted successfully"})
}

func (h *OrderHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	orders, err := h.service.List(r.Context())
	if err != nil {
		zap.L().Error("ListOrders failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, orders)
}

func (h *OrderHandler) ListUserOrders(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	requestingUserID, _ := middleware.GetUserIDFromContext(r.Context())
	role, _ := middleware.GetRoleFromContext(r.Context())

	if requestingUserID != userIDStr &&
		role != string(domain.Admin) &&
		role != string(domain.Manager) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	orders, err := h.service.ListByUser(r.Context(), userID)
	if err != nil {
		zap.L().Error("ListUserOrders failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, orders)
}

func (h *OrderHandler) GetOrderItems(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid order ID")
		return
	}

	items, err := h.service.GetOrderItems(r.Context(), id)
	if err != nil {
		zap.L().Error("GetOrderItems failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, items)
}
