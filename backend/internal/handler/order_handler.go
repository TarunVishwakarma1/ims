package handler

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/service"
	"github.com/TarunVishwakarma1/ims/backend/pkg/invoice"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
	"github.com/TarunVishwakarma1/ims/backend/pkg/utils"
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type OrderHandler struct {
	service         service.OrderService
	productService  service.ProductService
	invoiceRenderer *invoice.Renderer
	validate        *validator.Validate
}

func NewOrderHandler(service service.OrderService, productService service.ProductService) *OrderHandler {
	return &OrderHandler{
		service:         service,
		productService:  productService,
		invoiceRenderer: invoice.NewRenderer(),
		validate:        validator.New(),
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
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "organization not found in context")
		return
	}

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
		OrgID:  orgID,
		UserID: userID,
		Status: domain.OrderStatusPending,
	}

	var items []*domain.OrderItem
	for _, item := range req.Items {
		prod, err := h.productService.GetByID(r.Context(), item.ProductID, orgID)
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
			OrgID:     orgID,
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
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "organization not found in context")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid order ID")
		return
	}

	order, err := h.service.GetByID(r.Context(), id, orgID)
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
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "organization not found in context")
		return
	}

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

	if err := h.service.UpdateStatus(r.Context(), id, req.Status, orgID, ipAddress); err != nil {
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
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "organization not found in context")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid order ID")
		return
	}

	ipAddress := utils.GetClientIP(r)

	if err := h.service.Delete(r.Context(), id, orgID, ipAddress); err != nil {
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

// CancelOrder — POST /api/orders/{id}/cancel
// Body (optional): { "reason": "..." }
// Atomically marks order cancelled, releases reservations, restores stock.
func (h *OrderHandler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing org context")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid order id")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req) // body optional

	if err := h.service.Cancel(r.Context(), id, orgID, req.Reason, utils.GetClientIP(r)); err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeError(w, http.StatusNotFound, "order not found")
		case errors.Is(err, domain.ErrConflict):
			writeError(w, http.StatusConflict, "order cannot be cancelled in its current status")
		default:
			zap.L().Error("CancelOrder failed", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "internal server error: "+err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "order cancelled"})
}

func (h *OrderHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "organization not found in context")
		return
	}

	filters, err := parseOrderFilters(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.service.ListFiltered(r.Context(), orgID, filters)
	if err != nil {
		zap.L().Error("ListOrders failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ExportOrders streams a CSV of the filtered orders. Same query params as
// ListOrders except `page` / `per_page` are ignored (everything matched is
// exported). Capped at 10k rows server-side to avoid runaway responses.
func (h *OrderHandler) ExportOrders(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "organization not found in context")
		return
	}
	filters, err := parseOrderFilters(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// Export ignores client pagination — return up to 10k matches.
	filters.Page = 1
	filters.PerPage = 10000

	result, err := h.service.ListFiltered(r.Context(), orgID, filters)
	if err != nil {
		zap.L().Error("ExportOrders failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="orders-`+time.Now().UTC().Format("20060102-150405")+`.csv"`)

	cw := csv.NewWriter(w)
	defer cw.Flush()
	_ = cw.Write([]string{
		"id", "status", "payment_status", "order_type",
		"total_amount", "subtotal", "delivery_fee", "discount",
		"user_id", "buyer_org_id", "supplier_org_id",
		"created_at", "updated_at",
	})
	for _, o := range result.Items {
		_ = cw.Write([]string{
			o.ID.String(),
			string(o.Status),
			o.PaymentStatus,
			o.OrderType,
			strconv.FormatInt(o.TotalAmount, 10),
			strconv.FormatInt(o.Subtotal, 10),
			strconv.FormatInt(o.DeliveryFee, 10),
			strconv.FormatInt(o.Discount, 10),
			o.UserID.String(),
			uuidPtr(o.BuyerOrgID),
			uuidPtr(o.SupplierOrgID),
			o.CreatedAt.UTC().Format(time.RFC3339),
			o.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}
}

func uuidPtr(p *uuid.UUID) string {
	if p == nil {
		return ""
	}
	return p.String()
}

// parseOrderFilters reads OrderListFilters from query params. Empty params
// mean "no constraint" — returns a zero-valued struct in that case.
func parseOrderFilters(r *http.Request) (domain.OrderListFilters, error) {
	q := r.URL.Query()
	f := domain.OrderListFilters{
		Status:        q.Get("status"),
		PaymentStatus: q.Get("payment_status"),
		OrderType:     q.Get("order_type"),
		Search:        q.Get("search"),
	}
	if v := q.Get("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return f, errors.New("invalid page")
		}
		f.Page = n
	}
	if v := q.Get("per_page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return f, errors.New("invalid per_page")
		}
		f.PerPage = n
	}
	if v := q.Get("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return f, errors.New("from must be RFC3339")
		}
		f.From = &t
	}
	if v := q.Get("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return f, errors.New("to must be RFC3339")
		}
		f.To = &t
	}
	return f, nil
}

func (h *OrderHandler) ListUserOrders(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "organization not found in context")
		return
	}

	userIDStr := chi.URLParam(r, "user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	requestingUserID, _ := middleware.GetUserIDFromContext(r.Context())
	role, _ := middleware.GetRoleFromContext(r.Context())

	if requestingUserID != userIDStr &&
		role != "admin" &&
		role != "manager" {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	orders, err := h.service.ListByUser(r.Context(), userID, orgID)
	if err != nil {
		zap.L().Error("ListUserOrders failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, orders)
}

// BulkUpdateStatus — POST /api/orders/bulk-status
// Body: { ids: [uuid], status: "..." }
// Response: { applied: int, skipped: int }
func (h *OrderHandler) BulkUpdateStatus(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing org context")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		IDs    []uuid.UUID        `json:"ids"`
		Status domain.OrderStatus `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if len(req.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "ids required")
		return
	}
	if len(req.IDs) > 500 {
		writeError(w, http.StatusBadRequest, "max 500 ids per call")
		return
	}
	if req.Status == "" {
		writeError(w, http.StatusBadRequest, "status required")
		return
	}
	applied, skipped, err := h.service.BulkUpdateStatus(r.Context(), req.IDs, req.Status, orgID, utils.GetClientIP(r))
	if err != nil {
		zap.L().Error("BulkUpdateStatus failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"applied": applied, "skipped": skipped})
}

// CancelPreview — GET /api/orders/{id}/cancel-preview
// Returns the exact refund the backend would issue if the order were
// cancelled right now. UI uses this to render the cancel dialog without
// duplicating the policy table.
func (h *OrderHandler) CancelPreview(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing org context")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid order id")
		return
	}
	prev, err := h.service.CancelPreview(r.Context(), id, orgID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "order not found")
			return
		}
		zap.L().Error("CancelPreview failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, prev)
}

// Invoice — GET /api/orders/{id}/invoice.pdf
// Streams a PDF of the order. Reuses ProductService for name resolution
// so the PDF shows human-readable item names instead of UUIDs.
func (h *OrderHandler) Invoice(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing org context")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid order id")
		return
	}
	order, err := h.service.GetByID(r.Context(), id, orgID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "order not found")
			return
		}
		zap.L().Error("Invoice GetByID failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	items, err := h.service.GetOrderItems(r.Context(), id, orgID)
	if err != nil {
		zap.L().Error("Invoice GetOrderItems failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	// Resolve product names (best-effort — UUID fallback inside renderer).
	names := make(map[string]string, len(items))
	for _, it := range items {
		if p, err := h.productService.GetByID(r.Context(), it.ProductID, orgID); err == nil {
			names[it.ProductID.String()] = p.Name
		}
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition",
		`attachment; filename="invoice-`+order.ID.String()[:8]+`.pdf"`)
	if err := h.invoiceRenderer.Render(w, order, items, names); err != nil {
		zap.L().Error("invoice render failed", zap.Error(err))
		// Headers already sent — best we can do is log.
	}
}

// GetOrderTimeline — GET /api/orders/{id}/timeline
// Returns the audit-log trail for an order: created, status changes, cancel,
// etc. Chronological ascending.
func (h *OrderHandler) GetOrderTimeline(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing org context")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid order id")
		return
	}
	events, err := h.service.Timeline(r.Context(), id, orgID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "order not found")
			return
		}
		zap.L().Error("GetOrderTimeline failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if events == nil {
		events = []*domain.AuditLog{}
	}
	writeJSON(w, http.StatusOK, events)
}

func (h *OrderHandler) GetOrderItems(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "organization not found in context")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid order ID")
		return
	}

	items, err := h.service.GetOrderItems(r.Context(), id, orgID)
	if err != nil {
		zap.L().Error("GetOrderItems failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, items)
}
