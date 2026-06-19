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
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type InventoryService interface {
	Create(ctx context.Context, inventory *domain.Inventory, ipAddress string) error
	GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*domain.Inventory, error)
	GetByProductID(ctx context.Context, productID uuid.UUID, orgID uuid.UUID) (*domain.Inventory, error)
	Update(ctx context.Context, inventory *domain.Inventory) error
	Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID, ipAddress string) error
	List(ctx context.Context, orgID uuid.UUID) ([]*domain.Inventory, error)
	ListLowStock(ctx context.Context, orgID uuid.UUID) ([]*domain.Inventory, error)
	// Timeline returns chronological audit entries for an inventory row.
	// Backs the drill-down "stock-change history" view.
	Timeline(ctx context.Context, id, orgID uuid.UUID) ([]*domain.AuditLog, error)
}

type inventoryService struct {
	repo         repository.InventoryRepository
	auditLogRepo repository.AuditLogRepository
	cache        cache.Cache
	bus          events.Bus
}

func NewInventoryService(repo repository.InventoryRepository, auditLogRepo repository.AuditLogRepository, c cache.Cache, bus events.Bus) InventoryService {
	return &inventoryService{
		repo:         repo,
		auditLogRepo: auditLogRepo,
		cache:        c,
		bus:          bus,
	}
}

func (s *inventoryService) emit(ctx context.Context, inv *domain.Inventory) {
	_ = s.bus.Publish(ctx, events.NewEvent(events.TopicInventoryUpdated, inv.OrgID.String(), "", map[string]any{
		"id":                  inv.ID,
		"product_id":          inv.ProductID,
		"quantity":            inv.Quantity,
		"low_stock_threshold": inv.LowStockThreshold,
	}))
	// Low-stock alert if we just crossed the threshold
	if inv.Quantity <= inv.LowStockThreshold {
		_ = s.bus.Publish(ctx, events.NewEvent(events.TopicLowStockAlert, inv.OrgID.String(), "", map[string]any{
			"product_id": inv.ProductID,
			"quantity":   inv.Quantity,
			"threshold":  inv.LowStockThreshold,
		}))
	}
}

// Inventory mutations bust marketplace search (stock counts feed into it)
// and the per-org inventory list cache.
func (s *inventoryService) invalidate(ctx context.Context, orgID uuid.UUID) {
	_ = s.cache.DeleteByPattern(ctx, cache.MarketplaceSearchPattern())
	_ = s.cache.DeleteByPattern(ctx, cache.InventoryListPattern(orgID))
}

func (s *inventoryService) Create(ctx context.Context, inventory *domain.Inventory, ipAddress string) error {
	existing, err := s.repo.GetByProductID(ctx, inventory.ProductID, inventory.OrgID)
	if err == nil && existing != nil {
		return domain.ErrConflict
	}
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return err
	}

	inventory.ID = uuid.New()
	inventory.UpdatedAt = time.Now().UTC()

	if err := s.repo.Create(ctx, inventory); err != nil {
		return err
	}

	audit := &domain.AuditLog{
		ID:        uuid.New(),
		OrgID:     inventory.OrgID,
		UserID:    nil,
		Action:    "inventory.created",
		Entity:    "inventory",
		EntityID:  inventory.ID,
		IPAddress: ipAddress,
		CreatedAt: inventory.UpdatedAt,
	}
	if err := s.auditLogRepo.Create(ctx, audit); err != nil {
		zap.L().Error("audit log failed", zap.Error(err))
	}

	s.invalidate(ctx, inventory.OrgID)
	s.emit(ctx, inventory)
	return nil
}

func (s *inventoryService) GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*domain.Inventory, error) {
	return s.repo.GetByID(ctx, id, orgID)
}

func (s *inventoryService) GetByProductID(ctx context.Context, productID uuid.UUID, orgID uuid.UUID) (*domain.Inventory, error) {
	return s.repo.GetByProductID(ctx, productID, orgID)
}

func (s *inventoryService) Update(ctx context.Context, inventory *domain.Inventory) error {
	inventory.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, inventory); err != nil {
		return err
	}
	// Audit the stock change so the inventory drill-down can show history.
	audit := &domain.AuditLog{
		ID:        uuid.New(),
		OrgID:     inventory.OrgID,
		Action:    fmt.Sprintf("inventory.updated:qty=%d,threshold=%d", inventory.Quantity, inventory.LowStockThreshold),
		Entity:    "inventory",
		EntityID:  inventory.ID,
		CreatedAt: inventory.UpdatedAt,
	}
	if err := s.auditLogRepo.Create(ctx, audit); err != nil {
		zap.L().Warn("inventory audit log failed", zap.Error(err))
	}
	s.invalidate(ctx, inventory.OrgID)
	s.emit(ctx, inventory)
	return nil
}

func (s *inventoryService) Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID, ipAddress string) error {
	if err := s.repo.Delete(ctx, id, orgID); err != nil {
		return err
	}

	audit := &domain.AuditLog{
		ID:        uuid.New(),
		OrgID:     orgID,
		UserID:    nil,
		Action:    "inventory.deleted",
		Entity:    "inventory",
		EntityID:  id,
		IPAddress: ipAddress,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.auditLogRepo.Create(ctx, audit); err != nil {
		zap.L().Error("audit log failed", zap.Error(err))
	}

	s.invalidate(ctx, orgID)
	return nil
}

func (s *inventoryService) List(ctx context.Context, orgID uuid.UUID) ([]*domain.Inventory, error) {
	key := cache.InventoryListKey(orgID)
	var cached []*domain.Inventory
	if err := s.cache.Get(ctx, key, &cached); err == nil {
		return cached, nil
	}
	list, err := s.repo.List(ctx, orgID)
	if err != nil {
		return nil, err
	}
	_ = s.cache.Set(ctx, key, list, cache.TTLShort)
	return list, nil
}

func (s *inventoryService) Timeline(ctx context.Context, id, orgID uuid.UUID) ([]*domain.AuditLog, error) {
	// Verify ownership before exposing audit trail.
	if _, err := s.repo.GetByID(ctx, id, orgID); err != nil {
		return nil, err
	}
	return s.auditLogRepo.ListByEntity(ctx, "inventory", id, orgID)
}

func (s *inventoryService) ListLowStock(ctx context.Context, orgID uuid.UUID) ([]*domain.Inventory, error) {
	return s.repo.ListLowStock(ctx, orgID)
}
