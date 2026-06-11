package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/service"
	"github.com/TarunVishwakarma1/ims/backend/pkg/utils"
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type InventoryHandler struct {
	service  service.InventoryService
	validate *validator.Validate
}

func NewInventoryHandler(service service.InventoryService) *InventoryHandler {
	return &InventoryHandler{
		service:  service,
		validate: validator.New(),
	}
}

type CreateInventoryRequest struct {
	ProductID         uuid.UUID `json:"product_id" validate:"required"`
	Quantity          int       `json:"quantity" validate:"min=0"`
	LowStockThreshold int       `json:"low_stock_threshold" validate:"min=0"`
}

type UpdateInventoryRequest struct {
	ProductID         uuid.UUID `json:"product_id" validate:"required"`
	Quantity          int       `json:"quantity" validate:"min=0"`
	LowStockThreshold int       `json:"low_stock_threshold" validate:"min=0"`
}

func (h *InventoryHandler) CreateInventory(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB
	var req CreateInventoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ipAddress := utils.GetClientIP(r)

	inventory := &domain.Inventory{
		ProductID:         req.ProductID,
		Quantity:          req.Quantity,
		LowStockThreshold: req.LowStockThreshold,
	}

	if err := h.service.Create(r.Context(), inventory, ipAddress); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			writeError(w, http.StatusConflict, "inventory already exists for this product")
			return
		}
		zap.L().Error("CreateInventory failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusCreated, inventory)
}

func (h *InventoryHandler) GetInventoryByProduct(w http.ResponseWriter, r *http.Request) {
	productIDStr := chi.URLParam(r, "product_id")
	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid product ID")
		return
	}

	inventory, err := h.service.GetByProductID(r.Context(), productID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "inventory not found")
			return
		}
		zap.L().Error("GetInventoryByProduct failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, inventory)
}

func (h *InventoryHandler) UpdateInventory(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid inventory ID")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB
	var req UpdateInventoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	existing, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "inventory not found")
			return
		}
		zap.L().Error("UpdateInventory GetByID failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	existing.ProductID = req.ProductID
	existing.Quantity = req.Quantity
	existing.LowStockThreshold = req.LowStockThreshold

	if err := h.service.Update(r.Context(), existing); err != nil {
		zap.L().Error("UpdateInventory Update failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, existing)
}

func (h *InventoryHandler) ListInventory(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.List(r.Context())
	if err != nil {
		zap.L().Error("ListInventory failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, list)
}

func (h *InventoryHandler) ListLowStock(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.ListLowStock(r.Context())
	if err != nil {
		zap.L().Error("ListLowStock failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, list)
}
