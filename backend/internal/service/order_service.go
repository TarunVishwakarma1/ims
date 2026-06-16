package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/pkg/cache"
	"github.com/TarunVishwakarma1/ims/backend/pkg/events"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
	"github.com/TarunVishwakarma1/ims/backend/pkg/notify"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type OrderService interface {
	Create(ctx context.Context, order *domain.Order, items []*domain.OrderItem, ipAddress string) error
	GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*domain.Order, error)
	Update(ctx context.Context, order *domain.Order) error
	Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID, ipAddress string) error
	List(ctx context.Context, orgID uuid.UUID) ([]*domain.Order, error)

	// ListFiltered returns a paginated, filtered slice. Used by the
	// orders index page (server-side filter/search/pagination) and as the
	// data source for CSV export.
	ListFiltered(ctx context.Context, orgID uuid.UUID, f domain.OrderListFilters) (*domain.OrderListResult, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.OrderStatus, orgID uuid.UUID, ipAddress string) error
	ListByUser(ctx context.Context, userID uuid.UUID, orgID uuid.UUID) ([]*domain.Order, error)
	GetOrderItems(ctx context.Context, orderID uuid.UUID, orgID uuid.UUID) ([]*domain.OrderItem, error)

	// Timeline returns the audit log entries for an order in chronological
	// order — drives the UI status timeline.
	Timeline(ctx context.Context, orderID, orgID uuid.UUID) ([]*domain.AuditLog, error)

	// Cancel transitions a non-terminal order to cancelled, releases its
	// inventory reservations, and adds the reserved quantity back to inventory.
	// If the order was paid AND a PaymentService is wired, it also issues a
	// full refund. Returns ErrConflict on illegal transition.
	Cancel(ctx context.Context, id, orgID uuid.UUID, reason, ipAddress string) error

	// BulkUpdateStatus applies a status transition to many orders at once.
	// Returns counts of {applied, skipped} so the UI can show "5 updated,
	// 2 skipped (illegal transition or not found)". Per-row errors are
	// swallowed inside — the call only fails on infrastructure errors.
	BulkUpdateStatus(ctx context.Context, ids []uuid.UUID, status domain.OrderStatus, orgID uuid.UUID, ipAddress string) (applied int, skipped int, err error)

	// CancelPreview returns the canonical refund the backend would issue if
	// the order were cancelled right now. UI uses this to render the cancel
	// dialog — single source of truth, no policy mirror needed in the
	// frontend.
	CancelPreview(ctx context.Context, id, orgID uuid.UUID) (*domain.CancelPreview, error)

	// SetPaymentService wires the payment service post-construction to
	// avoid circular init between order and payment services.
	SetPaymentService(p PaymentOperations)
}

type orderService struct {
	repo          repository.OrderRepository
	inventoryRepo repository.InventoryRepository
	auditLogRepo  repository.AuditLogRepository
	marketRepo    repository.MarketplaceRepository
	bus           events.Bus
	cache         cache.Cache
	payments      PaymentOperations // optional; nil-safe — only RefundByOrder is used
	notifier      notify.Notifier
}

func NewOrderService(repo repository.OrderRepository, inventoryRepo repository.InventoryRepository, auditLogRepo repository.AuditLogRepository, marketRepo repository.MarketplaceRepository, bus events.Bus, c cache.Cache, n notify.Notifier) OrderService {
	return &orderService{
		repo:          repo,
		inventoryRepo: inventoryRepo,
		auditLogRepo:  auditLogRepo,
		marketRepo:    marketRepo,
		bus:           bus,
		cache:         c,
		notifier:      n,
	}
}

// SetPaymentService wires the payment service after construction. Decoupling
// avoids the circular-init problem between order and payment services.
// Takes the narrower PaymentOperations slice — order_service has no business
// poking at webhook internals.
func (s *orderService) SetPaymentService(p PaymentOperations) {
	s.payments = p
}

// cancellableStatuses are the statuses where Cancel will succeed. Mirror
// of the switch inside Cancel() — kept here so CancelPreview can answer
// "eligible?" without duplicating the logic.
func isCancellable(s domain.OrderStatus) bool {
	switch s {
	case domain.OrderStatusPending,
		domain.OrderStatusConfirmed,
		domain.OrderStatusAccepted,
		domain.OrderStatusProcessing,
		domain.OrderStatusReady:
		return true
	}
	return false
}

func (s *orderService) CancelPreview(ctx context.Context, id, orgID uuid.UUID) (*domain.CancelPreview, error) {
	order, err := s.repo.GetByID(ctx, id, orgID)
	if err != nil {
		return nil, err
	}
	prev := &domain.CancelPreview{
		Status:      order.Status,
		PaymentPaid: order.PaymentStatus == "paid",
		Eligible:    isCancellable(order.Status),
	}
	if !prev.Eligible {
		prev.Blocked = fmt.Sprintf("orders in %s status cannot be cancelled", order.Status)
		return prev, nil
	}
	prev.RefundPercent = domain.CancellationRefundPercent(order.Status)
	prev.Reason = domain.CancellationPolicyReason(order.Status)
	if prev.PaymentPaid {
		prev.RefundAmount = domain.ApplyRefundPercent(order.TotalAmount, prev.RefundPercent)
	}
	return prev, nil
}

// invalidateOrgOrders clears the orders cache for an org. Called after any
// mutation (create, status change, cancel, delete) so stale lists never leak.
func (s *orderService) invalidateOrgOrders(ctx context.Context, orgID uuid.UUID) {
	if err := s.cache.DeleteByPattern(ctx, cache.OrdersListPattern(orgID)); err != nil {
		zap.L().Warn("orders cache invalidate failed", zap.Error(err))
	}
}

// invalidateOrgInventory clears inventory list cache after stock changes
// performed inside this service (order create decrements, cancel restores).
func (s *orderService) invalidateOrgInventory(ctx context.Context, orgID uuid.UUID) {
	if err := s.cache.DeleteByPattern(ctx, cache.InventoryListPattern(orgID)); err != nil {
		zap.L().Warn("inventory cache invalidate failed", zap.Error(err))
	}
	_ = s.cache.DeleteByPattern(ctx, cache.MarketplaceSearchPattern())
}

// actorFromContext returns the user UUID stored in context by Auth middleware,
// or nil if absent / unparseable. Used to attribute audit log entries.
func actorFromContext(ctx context.Context) *uuid.UUID {
	v := ctx.Value(middleware.ContextKeyUserID)
	s, ok := v.(string)
	if !ok || s == "" {
		return nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil
	}
	return &id
}

// Cancel atomically cancels an order, releases its inventory reservations,
// and restores the reserved quantity to inventory. Only valid for non-terminal,
// non-shipped statuses.
// cancelTx wraps the transactional side of Cancel: release reservations,
// restock inventory, flip the order to cancelled. Pulled out so Cancel's
// body focuses on pre-conditions + post-commit side effects (audit, cache
// invalidation, refund, events, notifications).
func (s *orderService) cancelTx(ctx context.Context, order *domain.Order) error {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	txOrder := s.repo.WithTx(tx)
	txInv := s.inventoryRepo.WithTx(tx)
	txMarket := s.marketRepo.WithTx(tx)

	released, err := txMarket.ReleaseByOrder(ctx, order.ID)
	if err != nil {
		return err
	}
	for _, res := range released {
		inv, err := txInv.GetByID(ctx, res.InventoryID, res.OrgID)
		if err != nil {
			return err
		}
		inv.Quantity += res.Quantity
		inv.UpdatedAt = time.Now().UTC()
		if err := txInv.Update(ctx, inv); err != nil {
			return err
		}
	}

	now := time.Now().UTC()
	order.Status = domain.OrderStatusCancelled
	order.CancelledAt = &now
	order.UpdatedAt = now
	if err := txOrder.Update(ctx, order); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *orderService) Cancel(ctx context.Context, id, orgID uuid.UUID, reason, ipAddress string) error {
	order, err := s.repo.GetByID(ctx, id, orgID)
	if err != nil {
		return err
	}
	if !isCancellable(order.Status) {
		return domain.ErrConflict
	}
	// Remember the status before we flip it — refund policy depends on it.
	preCancelStatus := order.Status

	if err := s.cancelTx(ctx, order); err != nil {
		return err
	}
	now := order.UpdatedAt // set inside cancelTx

	audit := &domain.AuditLog{
		ID:        uuid.New(),
		OrgID:     orgID,
		UserID:    actorFromContext(ctx),
		Action:    "order.cancelled",
		Entity:    "orders",
		EntityID:  order.ID,
		IPAddress: ipAddress,
		CreatedAt: now,
	}
	if err := s.auditLogRepo.Create(ctx, audit); err != nil {
		zap.L().Error("audit log failed", zap.Error(err))
	}

	s.invalidateOrgOrders(ctx, orgID)
	s.invalidateOrgInventory(ctx, orgID)

	// Auto-refund: if the order was paid, apply the cancellation refund
	// policy. Percentage shrinks as the order progresses (Zomato/Blinkit
	// model — see domain.CancellationRefundPercent). Failures here are
	// logged but don't undo the cancel — operator can retry from the
	// order detail page.
	if order.PaymentStatus == "paid" && s.payments != nil {
		// Use the PRE-cancel status (captured before the flip above) so
		// the refund percent matches what the customer saw in the dialog.
		percent := domain.CancellationRefundPercent(preCancelStatus)
		refundAmount := domain.ApplyRefundPercent(order.TotalAmount, percent)

		refundReason := reason
		if refundReason == "" {
			refundReason = "order cancelled"
		}
		refundReason = fmt.Sprintf("%s (%d%% policy: %s)",
			refundReason, percent, domain.CancellationPolicyReason(preCancelStatus))

		if refundAmount > 0 {
			if err := s.payments.RefundByOrder(ctx, orgID, order.ID, refundAmount, refundReason); err != nil {
				zap.L().Warn("auto-refund after cancel failed",
					zap.String("order_id", order.ID.String()),
					zap.Int64("amount", refundAmount),
					zap.Error(err))
			}
		} else {
			zap.L().Info("cancel with no refund per policy",
				zap.String("order_id", order.ID.String()),
				zap.String("status", string(preCancelStatus)))
		}
	}

	_ = s.bus.Publish(ctx, events.NewEvent(events.TopicOrderStatusChanged, orgID.String(), "", map[string]any{
		"id":     order.ID,
		"status": domain.OrderStatusCancelled,
		"reason": reason,
	}))
	if s.notifier != nil {
		s.notifier.OrderCancelled(ctx, order, reason)
	}
	return nil
}

// createTx is the transactional core of Create: validate + decrement stock,
// insert the order row, insert order items. Returns the resolved totalAmount
// so the caller can stamp it on the order before publishing events.
func (s *orderService) createTx(ctx context.Context, order *domain.Order, items []*domain.OrderItem, now time.Time) error {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	txOrderRepo := s.repo.WithTx(tx)
	txInventoryRepo := s.inventoryRepo.WithTx(tx)

	var totalAmount int64
	for _, item := range items {
		inv, err := txInventoryRepo.GetByProductID(ctx, item.ProductID, order.OrgID)
		if err != nil {
			return err
		}
		if inv.Quantity < item.Quantity {
			return domain.ErrInsufficientStock
		}
		inv.Quantity -= item.Quantity
		inv.UpdatedAt = now
		if err := txInventoryRepo.Update(ctx, inv); err != nil {
			return err
		}
		totalAmount += int64(item.Quantity) * item.UnitPrice
	}
	order.TotalAmount = totalAmount

	if err := txOrderRepo.Create(ctx, order); err != nil {
		return err
	}
	for _, item := range items {
		item.ID = uuid.New()
		item.OrderID = order.ID
		item.OrgID = order.OrgID
		if err := txOrderRepo.CreateOrderItem(ctx, item); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *orderService) Create(ctx context.Context, order *domain.Order, items []*domain.OrderItem, ipAddress string) error {
	order.ID = uuid.New()
	now := time.Now().UTC()
	order.CreatedAt = now
	order.UpdatedAt = now

	if err := s.createTx(ctx, order, items, now); err != nil {
		return err
	}

	audit := &domain.AuditLog{
		ID:        uuid.New(),
		OrgID:     order.OrgID,
		UserID:    &order.UserID,
		Action:    "order.created",
		Entity:    "orders",
		EntityID:  order.ID,
		IPAddress: ipAddress,
		CreatedAt: now,
	}
	if err := s.auditLogRepo.Create(ctx, audit); err != nil {
		zap.L().Error("audit log failed", zap.Error(err))
	}

	s.invalidateOrgOrders(ctx, order.OrgID)
	s.invalidateOrgInventory(ctx, order.OrgID)

	_ = s.bus.Publish(ctx, events.NewEvent(events.TopicOrderCreated, order.OrgID.String(), order.UserID.String(), map[string]any{
		"id":           order.ID,
		"status":       order.Status,
		"total_amount": order.TotalAmount,
	}))
	if s.notifier != nil {
		s.notifier.OrderCreated(ctx, order)
	}
	return nil
}

func (s *orderService) GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*domain.Order, error) {
	return s.repo.GetByID(ctx, id, orgID)
}

func (s *orderService) Update(ctx context.Context, order *domain.Order) error {
	order.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, order); err != nil {
		return err
	}
	s.invalidateOrgOrders(ctx, order.OrgID)
	return nil
}

func (s *orderService) Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID, ipAddress string) error {
	if err := s.repo.Delete(ctx, id, orgID); err != nil {
		return err
	}
	s.invalidateOrgOrders(ctx, orgID)

	audit := &domain.AuditLog{
		ID:        uuid.New(),
		OrgID:     orgID,
		UserID:    actorFromContext(ctx),
		Action:    "order.deleted",
		Entity:    "orders",
		EntityID:  id,
		IPAddress: ipAddress,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.auditLogRepo.Create(ctx, audit); err != nil {
		zap.L().Error("audit log failed", zap.Error(err))
	}

	return nil
}

func (s *orderService) List(ctx context.Context, orgID uuid.UUID) ([]*domain.Order, error) {
	key := cache.OrdersListKey(orgID)
	var cached []*domain.Order
	if err := s.cache.Get(ctx, key, &cached); err == nil {
		return cached, nil
	}
	orders, err := s.repo.List(ctx, orgID)
	if err != nil {
		return nil, err
	}
	_ = s.cache.Set(ctx, key, orders, cache.TTLShort)
	return orders, nil
}

// ListFiltered intentionally bypasses the cache — the filter combination
// space is too large to cache effectively, and the SQL is already paginated.
func (s *orderService) ListFiltered(ctx context.Context, orgID uuid.UUID, f domain.OrderListFilters) (*domain.OrderListResult, error) {
	items, total, err := s.repo.ListFiltered(ctx, orgID, f)
	if err != nil {
		return nil, err
	}
	page := f.Page
	if page < 1 {
		page = 1
	}
	per := f.PerPage
	if per <= 0 {
		per = 25
	}
	return &domain.OrderListResult{
		Items:   items,
		Total:   total,
		Page:    page,
		PerPage: per,
	}, nil
}

func (s *orderService) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.OrderStatus, orgID uuid.UUID, ipAddress string) error {
	existing, err := s.repo.GetByID(ctx, id, orgID)
	if err != nil {
		return err
	}

	valid := false
	switch existing.Status {
	case domain.OrderStatusPending:
		valid = status == domain.OrderStatusAccepted || status == domain.OrderStatusRejected || status == domain.OrderStatusCancelled
	case domain.OrderStatusAccepted:
		valid = status == domain.OrderStatusProcessing || status == domain.OrderStatusCancelled
	case domain.OrderStatusProcessing:
		valid = status == domain.OrderStatusReady || status == domain.OrderStatusCancelled
	case domain.OrderStatusReady:
		valid = status == domain.OrderStatusShipped || status == domain.OrderStatusCancelled
	case domain.OrderStatusShipped:
		valid = status == domain.OrderStatusDelivered
	case domain.OrderStatusDelivered:
		valid = status == domain.OrderStatusCompleted
	case domain.OrderStatusConfirmed:
		valid = status == domain.OrderStatusAccepted || status == domain.OrderStatusCancelled
	}

	if !valid {
		return domain.ErrConflict
	}

	if err := s.repo.UpdateStatus(ctx, id, status, orgID); err != nil {
		return err
	}
	s.invalidateOrgOrders(ctx, orgID)

	audit := &domain.AuditLog{
		ID:        uuid.New(),
		OrgID:     orgID,
		UserID:    actorFromContext(ctx),
		Action:    "order.status_updated:" + string(status),
		Entity:    "orders",
		EntityID:  id,
		IPAddress: ipAddress,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.auditLogRepo.Create(ctx, audit); err != nil {
		zap.L().Error("audit log failed", zap.Error(err))
	}

	_ = s.bus.Publish(ctx, events.NewEvent(events.TopicOrderStatusChanged, orgID.String(), "", map[string]any{
		"id":     id,
		"status": status,
	}))
	if s.notifier != nil {
		// Refetch the order so the notifier sees the new status (existing
		// pointer reflects pre-update state).
		if updated, err := s.repo.GetByID(ctx, id, orgID); err == nil {
			s.notifier.OrderStatusChanged(ctx, updated)
		}
	}
	return nil
}

// BulkUpdateStatus loops UpdateStatus for each id. Single-org scope, so
// ownership check is implicit. Skipped reasons (not found / illegal
// transition) are aggregated as counts to keep the response simple; UI
// shows toast like "5 updated, 2 skipped".
func (s *orderService) BulkUpdateStatus(ctx context.Context, ids []uuid.UUID, status domain.OrderStatus, orgID uuid.UUID, ipAddress string) (int, int, error) {
	applied := 0
	skipped := 0
	for _, id := range ids {
		if err := s.UpdateStatus(ctx, id, status, orgID, ipAddress); err != nil {
			if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrConflict) {
				skipped++
				continue
			}
			// Infra error — bail; partial results already applied stay applied.
			return applied, skipped, err
		}
		applied++
	}
	return applied, skipped, nil
}

func (s *orderService) ListByUser(ctx context.Context, userID uuid.UUID, orgID uuid.UUID) ([]*domain.Order, error) {
	return s.repo.ListByUser(ctx, userID, orgID)
}

func (s *orderService) GetOrderItems(ctx context.Context, orderID uuid.UUID, orgID uuid.UUID) ([]*domain.OrderItem, error) {
	return s.repo.GetOrderItems(ctx, orderID, orgID)
}

func (s *orderService) Timeline(ctx context.Context, orderID, orgID uuid.UUID) ([]*domain.AuditLog, error) {
	// First make sure the order belongs to this org (prevents leaking audit
	// entries via guessed IDs).
	if _, err := s.repo.GetByID(ctx, orderID, orgID); err != nil {
		return nil, err
	}
	return s.auditLogRepo.ListByEntity(ctx, "orders", orderID, orgID)
}
