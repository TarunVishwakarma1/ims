package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/pkg/events"
	"github.com/TarunVishwakarma1/ims/backend/pkg/notify"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Topics for return lifecycle — wired into SSE so the UI updates live.
const (
	TopicReturnCreated  = "return.created"
	TopicReturnApproved = "return.approved"
	TopicReturnRejected = "return.rejected"
	TopicReturnReceived = "return.received"
	TopicReturnRefunded = "return.refunded"
)

type ReturnService interface {
	// Create opens a new return request for an order. Validates order is
	// eligible (delivered/completed), items belong to the order, requested
	// quantities don't exceed ordered quantities. Computes refund_amount
	// from item × unit_price (frozen at request time).
	Create(ctx context.Context, orgID, userID uuid.UUID, req CreateReturnInput) (*domain.ReturnRequest, error)

	GetByID(ctx context.Context, id, orgID uuid.UUID) (*domain.ReturnRequest, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]*domain.ReturnRequest, error)
	ListByOrder(ctx context.Context, orderID, orgID uuid.UUID) ([]*domain.ReturnRequest, error)

	Approve(ctx context.Context, id, orgID uuid.UUID) error
	Reject(ctx context.Context, id, orgID uuid.UUID, note string) error

	// MarkReceived transitions to `received`, restores stock for the
	// returned items, then auto-triggers the refund.
	MarkReceived(ctx context.Context, id, orgID uuid.UUID) error
}

type CreateReturnInput struct {
	OrderID uuid.UUID
	Reason  string
	Items   []CreateReturnItem
}

type CreateReturnItem struct {
	OrderItemID uuid.UUID
	Quantity    int
}

type returnService struct {
	repo          repository.ReturnRepository
	orderRepo     repository.OrderRepository
	inventoryRepo repository.InventoryRepository
	auditRepo     repository.AuditLogRepository
	payments      PaymentService
	bus           events.Bus
	notifier      notify.Notifier
}

func NewReturnService(
	r repository.ReturnRepository,
	orderRepo repository.OrderRepository,
	inv repository.InventoryRepository,
	audit repository.AuditLogRepository,
	payments PaymentService,
	bus events.Bus,
	n notify.Notifier,
) ReturnService {
	return &returnService{
		repo:          r,
		orderRepo:     orderRepo,
		inventoryRepo: inv,
		auditRepo:     audit,
		payments:      payments,
		bus:           bus,
		notifier:      n,
	}
}

func (s *returnService) Create(ctx context.Context, orgID, userID uuid.UUID, in CreateReturnInput) (*domain.ReturnRequest, error) {
	if len(in.Items) == 0 {
		return nil, errors.New("at least one return item is required")
	}
	if in.Reason == "" {
		return nil, errors.New("reason is required")
	}

	order, err := s.orderRepo.GetByID(ctx, in.OrderID, orgID)
	if err != nil {
		return nil, err
	}
	// Only delivered or completed orders are eligible. Cancelled/refunded
	// orders use the cancel flow, not the return flow.
	if order.Status != domain.OrderStatusDelivered && order.Status != domain.OrderStatusCompleted {
		return nil, fmt.Errorf("order status %s is not eligible for return", order.Status)
	}

	orderItems, err := s.orderRepo.GetOrderItems(ctx, order.ID, orgID)
	if err != nil {
		return nil, err
	}
	orderItemByID := make(map[uuid.UUID]*domain.OrderItem, len(orderItems))
	for _, it := range orderItems {
		orderItemByID[it.ID] = it
	}

	// Prior in-flight + completed returns on this order — caps the
	// remaining returnable quantity per order item. Rejected returns are
	// excluded by the query.
	priorReturned, err := s.repo.PriorReturnedByItem(ctx, order.ID)
	if err != nil {
		return nil, err
	}

	// Compute refund + validate every requested item belongs to this order
	// AND that ordered_qty - already_returned_qty >= requested_qty.
	var refund int64
	resolved := make([]*domain.ReturnItem, 0, len(in.Items))
	for _, ri := range in.Items {
		oi, ok := orderItemByID[ri.OrderItemID]
		if !ok {
			return nil, fmt.Errorf("order item %s does not belong to order", ri.OrderItemID)
		}
		if ri.Quantity <= 0 {
			return nil, fmt.Errorf("return quantity must be > 0 for item %s", ri.OrderItemID)
		}
		alreadyReturned := priorReturned[oi.ID]
		remaining := oi.Quantity - alreadyReturned
		if ri.Quantity > remaining {
			return nil, fmt.Errorf(
				"return quantity %d exceeds remaining %d for item %s (ordered %d, already returned %d)",
				ri.Quantity, remaining, oi.ID, oi.Quantity, alreadyReturned)
		}
		refund += int64(ri.Quantity) * oi.UnitPrice
		resolved = append(resolved, &domain.ReturnItem{
			ID:          uuid.New(),
			OrderItemID: oi.ID,
			ProductID:   oi.ProductID,
			Quantity:    ri.Quantity,
			UnitPrice:   oi.UnitPrice,
		})
	}

	now := time.Now().UTC()
	req := &domain.ReturnRequest{
		ID:           uuid.New(),
		OrgID:        orgID,
		OrderID:      in.OrderID,
		RequestedBy:  userID,
		Status:       domain.ReturnStatusRequested,
		Reason:       in.Reason,
		RefundAmount: refund,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	txRepo := s.repo.WithTx(tx)

	if err := txRepo.Create(ctx, req); err != nil {
		return nil, err
	}
	for _, it := range resolved {
		it.ReturnID = req.ID
		if err := txRepo.CreateItem(ctx, it); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	req.Items = resolved
	s.writeAudit(ctx, orgID, &userID, req.OrderID, "return.requested")
	_ = s.bus.Publish(ctx, events.NewEvent(TopicReturnCreated, orgID.String(), userID.String(), map[string]any{
		"id":            req.ID,
		"order_id":      req.OrderID,
		"refund_amount": req.RefundAmount,
	}))
	if s.notifier != nil {
		s.notifier.ReturnCreated(ctx, req, order)
	}
	// Publish an org-scoped event for the SUPPLIER side too, so the
	// supplier's web SSE subscriber sees the inbound RMA in real time.
	// Skipped when buyer and supplier are the same org.
	if order.SupplierOrgID != nil && *order.SupplierOrgID != orgID {
		_ = s.bus.Publish(ctx, events.NewEvent(TopicReturnCreated, order.SupplierOrgID.String(), "", map[string]any{
			"id":            req.ID,
			"order_id":      req.OrderID,
			"refund_amount": req.RefundAmount,
			"direction":     "incoming",
		}))
	}
	return req, nil
}

func (s *returnService) GetByID(ctx context.Context, id, orgID uuid.UUID) (*domain.ReturnRequest, error) {
	return s.repo.GetByID(ctx, id, orgID)
}

func (s *returnService) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]*domain.ReturnRequest, error) {
	return s.repo.ListByOrg(ctx, orgID)
}

func (s *returnService) ListByOrder(ctx context.Context, orderID, orgID uuid.UUID) ([]*domain.ReturnRequest, error) {
	return s.repo.ListByOrder(ctx, orderID, orgID)
}

func (s *returnService) transition(ctx context.Context, id, orgID uuid.UUID, to domain.ReturnStatus, note *string) (*domain.ReturnRequest, error) {
	req, err := s.repo.GetByID(ctx, id, orgID)
	if err != nil {
		return nil, err
	}
	if !domain.CanTransitionReturn(req.Status, to) {
		return nil, domain.ErrConflict
	}
	if err := s.repo.UpdateStatus(ctx, id, to, note); err != nil {
		return nil, err
	}
	req.Status = to
	return req, nil
}

func (s *returnService) Approve(ctx context.Context, id, orgID uuid.UUID) error {
	req, err := s.transition(ctx, id, orgID, domain.ReturnStatusApproved, nil)
	if err != nil {
		return err
	}
	s.writeAudit(ctx, orgID, actorFromContext(ctx), req.OrderID, "return.approved")
	_ = s.bus.Publish(ctx, events.NewEvent(TopicReturnApproved, orgID.String(), "", map[string]any{
		"id":       req.ID,
		"order_id": req.OrderID,
	}))
	return nil
}

func (s *returnService) Reject(ctx context.Context, id, orgID uuid.UUID, note string) error {
	if note == "" {
		return errors.New("rejection note is required")
	}
	req, err := s.transition(ctx, id, orgID, domain.ReturnStatusRejected, &note)
	if err != nil {
		return err
	}
	s.writeAudit(ctx, orgID, actorFromContext(ctx), req.OrderID, "return.rejected")
	_ = s.bus.Publish(ctx, events.NewEvent(TopicReturnRejected, orgID.String(), "", map[string]any{
		"id":       req.ID,
		"order_id": req.OrderID,
		"reason":   note,
	}))
	return nil
}

// MarkReceived flips the request to `received`, restocks inventory for the
// returned units, and triggers a refund. The refund + restock happen in
// the same transaction so partial failure rolls back cleanly. Refund call
// runs AFTER commit to avoid holding a DB tx across a network round-trip.
func (s *returnService) MarkReceived(ctx context.Context, id, orgID uuid.UUID) error {
	req, err := s.repo.GetByID(ctx, id, orgID)
	if err != nil {
		return err
	}
	if !domain.CanTransitionReturn(req.Status, domain.ReturnStatusReceived) {
		// Allow direct approved → received in case in_transit was skipped.
		if req.Status != domain.ReturnStatusApproved {
			return domain.ErrConflict
		}
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	txRepo := s.repo.WithTx(tx)
	txInv := s.inventoryRepo.WithTx(tx)

	// Restock — add quantities back to supplier-org inventory.
	// We look up by product_id within the order's supplier org (or own org
	// for internal orders). This loses location granularity; acceptable for
	// v1, can be refined when we track per-location reservations.
	order, err := s.orderRepo.GetByID(ctx, req.OrderID, orgID)
	if err != nil {
		return err
	}
	supplier := orgID
	if order.SupplierOrgID != nil {
		supplier = *order.SupplierOrgID
	}
	for _, it := range req.Items {
		inv, err := txInv.GetByProductID(ctx, it.ProductID, supplier)
		if err != nil {
			return fmt.Errorf("inventory lookup failed for product %s: %w", it.ProductID, err)
		}
		inv.Quantity += it.Quantity
		inv.UpdatedAt = time.Now().UTC()
		if err := txInv.Update(ctx, inv); err != nil {
			return err
		}
	}
	if err := txRepo.UpdateStatus(ctx, id, domain.ReturnStatusReceived, nil); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	s.writeAudit(ctx, orgID, actorFromContext(ctx), req.OrderID, "return.received")
	_ = s.bus.Publish(ctx, events.NewEvent(TopicReturnReceived, orgID.String(), "", map[string]any{
		"id":       req.ID,
		"order_id": req.OrderID,
	}))

	// Auto-refund AFTER commit so a slow RazorPay call doesn't hold the tx.
	// Failures are logged; operator can retry via the detail page.
	if req.RefundAmount > 0 && s.payments != nil {
		if err := s.payments.RefundByOrder(ctx, orgID, req.OrderID, req.RefundAmount, "return received: "+req.Reason); err != nil {
			zap.L().Warn("return refund failed",
				zap.String("return_id", req.ID.String()),
				zap.Error(err))
			return nil // return is still received; refund retry is separate
		}
		// Mark refunded once the refund call returned without error.
		if err := s.repo.UpdateStatus(ctx, id, domain.ReturnStatusRefunded, nil); err != nil {
			zap.L().Warn("failed to mark return refunded", zap.Error(err))
			return nil
		}
		s.writeAudit(ctx, orgID, actorFromContext(ctx), req.OrderID, "return.refunded")
		_ = s.bus.Publish(ctx, events.NewEvent(TopicReturnRefunded, orgID.String(), "", map[string]any{
			"id":       req.ID,
			"order_id": req.OrderID,
		}))
		if s.notifier != nil {
			s.notifier.ReturnRefunded(ctx, req, order)
		}
	}
	return nil
}

func (s *returnService) writeAudit(ctx context.Context, orgID uuid.UUID, userID *uuid.UUID, orderID uuid.UUID, action string) {
	entry := &domain.AuditLog{
		ID:        uuid.New(),
		OrgID:     orgID,
		UserID:    userID,
		Action:    action,
		Entity:    "orders",
		EntityID:  orderID,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.auditRepo.Create(ctx, entry); err != nil {
		zap.L().Warn("return audit log failed", zap.Error(err))
	}
}
