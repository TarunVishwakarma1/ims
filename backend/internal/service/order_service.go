package service

import (
	"context"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/pkg/events"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type OrderService interface {
	Create(ctx context.Context, order *domain.Order, items []*domain.OrderItem, ipAddress string) error
	GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*domain.Order, error)
	Update(ctx context.Context, order *domain.Order) error
	Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID, ipAddress string) error
	List(ctx context.Context, orgID uuid.UUID) ([]*domain.Order, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.OrderStatus, orgID uuid.UUID, ipAddress string) error
	ListByUser(ctx context.Context, userID uuid.UUID, orgID uuid.UUID) ([]*domain.Order, error)
	GetOrderItems(ctx context.Context, orderID uuid.UUID, orgID uuid.UUID) ([]*domain.OrderItem, error)
}

type orderService struct {
	repo          repository.OrderRepository
	inventoryRepo repository.InventoryRepository
	auditLogRepo  repository.AuditLogRepository
	bus           events.Bus
}

func NewOrderService(repo repository.OrderRepository, inventoryRepo repository.InventoryRepository, auditLogRepo repository.AuditLogRepository, bus events.Bus) OrderService {
	return &orderService{
		repo:          repo,
		inventoryRepo: inventoryRepo,
		auditLogRepo:  auditLogRepo,
		bus:           bus,
	}
}

func (s *orderService) Create(ctx context.Context, order *domain.Order, items []*domain.OrderItem, ipAddress string) error {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	txOrderRepo := s.repo.WithTx(tx)
	txInventoryRepo := s.inventoryRepo.WithTx(tx)

	order.ID = uuid.New()
	now := time.Now().UTC()
	order.CreatedAt = now
	order.UpdatedAt = now

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

	if err := tx.Commit(ctx); err != nil {
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

	_ = s.bus.Publish(ctx, events.NewEvent(events.TopicOrderCreated, order.OrgID.String(), order.UserID.String(), map[string]any{
		"id":           order.ID,
		"status":       order.Status,
		"total_amount": order.TotalAmount,
	}))
	return nil
}

func (s *orderService) GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*domain.Order, error) {
	return s.repo.GetByID(ctx, id, orgID)
}

func (s *orderService) Update(ctx context.Context, order *domain.Order) error {
	order.UpdatedAt = time.Now().UTC()
	return s.repo.Update(ctx, order)
}

func (s *orderService) Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID, ipAddress string) error {
	if err := s.repo.Delete(ctx, id, orgID); err != nil {
		return err
	}

	audit := &domain.AuditLog{
		ID:        uuid.New(),
		OrgID:     orgID,
		UserID:    nil,
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
	return s.repo.List(ctx, orgID)
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

	audit := &domain.AuditLog{
		ID:        uuid.New(),
		OrgID:     orgID,
		UserID:    nil,
		Action:    "order.status_updated",
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
	return nil
}

func (s *orderService) ListByUser(ctx context.Context, userID uuid.UUID, orgID uuid.UUID) ([]*domain.Order, error) {
	return s.repo.ListByUser(ctx, userID, orgID)
}

func (s *orderService) GetOrderItems(ctx context.Context, orderID uuid.UUID, orgID uuid.UUID) ([]*domain.OrderItem, error) {
	return s.repo.GetOrderItems(ctx, orderID, orgID)
}
