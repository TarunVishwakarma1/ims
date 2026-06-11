package service

import (
	"context"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type OrderService interface {
	Create(ctx context.Context, order *domain.Order, items []*domain.OrderItem, ipAddress string) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Order, error)
	Update(ctx context.Context, order *domain.Order) error
	Delete(ctx context.Context, id uuid.UUID, ipAddress string) error
	List(ctx context.Context) ([]*domain.Order, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.OrderStatus, ipAddress string) error
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*domain.Order, error)
	GetOrderItems(ctx context.Context, orderID uuid.UUID) ([]*domain.OrderItem, error)
}

type orderService struct {
	repo          repository.OrderRepository
	inventoryRepo repository.InventoryRepository
	auditLogRepo  repository.AuditLogRepository
}

func NewOrderService(repo repository.OrderRepository, inventoryRepo repository.InventoryRepository, auditLogRepo repository.AuditLogRepository) OrderService {
	return &orderService{
		repo:          repo,
		inventoryRepo: inventoryRepo,
		auditLogRepo:  auditLogRepo,
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
		inv, err := txInventoryRepo.GetByProductID(ctx, item.ProductID)
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
		if err := txOrderRepo.CreateOrderItem(ctx, item); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	audit := &domain.AuditLog{
		ID:        uuid.New(),
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

	return nil
}

func (s *orderService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *orderService) Update(ctx context.Context, order *domain.Order) error {
	order.UpdatedAt = time.Now().UTC()
	return s.repo.Update(ctx, order)
}

func (s *orderService) Delete(ctx context.Context, id uuid.UUID, ipAddress string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	audit := &domain.AuditLog{
		ID:        uuid.New(),
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

func (s *orderService) List(ctx context.Context) ([]*domain.Order, error) {
	return s.repo.List(ctx)
}

func (s *orderService) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.OrderStatus, ipAddress string) error {
	if err := s.repo.UpdateStatus(ctx, id, status); err != nil {
		return err
	}

	audit := &domain.AuditLog{
		ID:        uuid.New(),
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

	return nil
}

func (s *orderService) ListByUser(ctx context.Context, userID uuid.UUID) ([]*domain.Order, error) {
	return s.repo.ListByUser(ctx, userID)
}

func (s *orderService) GetOrderItems(ctx context.Context, orderID uuid.UUID) ([]*domain.OrderItem, error) {
	return s.repo.GetOrderItems(ctx, orderID)
}
