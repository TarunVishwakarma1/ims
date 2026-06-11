package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/google/uuid"
)

type InventoryService interface {
	Create(ctx context.Context, inventory *domain.Inventory, ipAddress string) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Inventory, error)
	GetByProductID(ctx context.Context, productID uuid.UUID) (*domain.Inventory, error)
	Update(ctx context.Context, inventory *domain.Inventory) error
	Delete(ctx context.Context, id uuid.UUID, ipAddress string) error
	List(ctx context.Context) ([]*domain.Inventory, error)
	ListLowStock(ctx context.Context) ([]*domain.Inventory, error)
}

type inventoryService struct {
	repo         repository.InventoryRepository
	auditLogRepo repository.AuditLogRepository
}

func NewInventoryService(repo repository.InventoryRepository, auditLogRepo repository.AuditLogRepository) InventoryService {
	return &inventoryService{
		repo:         repo,
		auditLogRepo: auditLogRepo,
	}
}

func (s *inventoryService) Create(ctx context.Context, inventory *domain.Inventory, ipAddress string) error {
	existing, err := s.repo.GetByProductID(ctx, inventory.ProductID)
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
		UserID:    nil,
		Action:    "inventory.created",
		Entity:    "inventory",
		EntityID:  inventory.ID,
		IPAddress: ipAddress,
		CreatedAt: inventory.UpdatedAt,
	}
	if err := s.auditLogRepo.Create(ctx, audit); err != nil {
		fmt.Printf("audit log failed: %v\n", err)
	}

	return nil
}

func (s *inventoryService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Inventory, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *inventoryService) GetByProductID(ctx context.Context, productID uuid.UUID) (*domain.Inventory, error) {
	return s.repo.GetByProductID(ctx, productID)
}

func (s *inventoryService) Update(ctx context.Context, inventory *domain.Inventory) error {
	inventory.UpdatedAt = time.Now().UTC()
	return s.repo.Update(ctx, inventory)
}

func (s *inventoryService) Delete(ctx context.Context, id uuid.UUID, ipAddress string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	audit := &domain.AuditLog{
		ID:        uuid.New(),
		UserID:    nil,
		Action:    "inventory.deleted",
		Entity:    "inventory",
		EntityID:  id,
		IPAddress: ipAddress,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.auditLogRepo.Create(ctx, audit); err != nil {
		fmt.Printf("audit log failed: %v\n", err)
	}

	return nil
}

func (s *inventoryService) List(ctx context.Context) ([]*domain.Inventory, error) {
	return s.repo.List(ctx)
}

func (s *inventoryService) ListLowStock(ctx context.Context) ([]*domain.Inventory, error) {
	return s.repo.ListLowStock(ctx)
}
