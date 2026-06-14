package service

import (
	"context"
	"errors"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/pkg/cache"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type ProductService interface {
	Create(ctx context.Context, product *domain.Product, ipAddress string) error
	GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*domain.Product, error)
	GetBySKU(ctx context.Context, sku string, orgID uuid.UUID) (*domain.Product, error)
	Update(ctx context.Context, product *domain.Product) error
	Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID, ipAddress string) error
	List(ctx context.Context, orgID uuid.UUID) ([]*domain.Product, error)
	ListByCategory(ctx context.Context, categoryID uuid.UUID, orgID uuid.UUID) ([]*domain.Product, error)
}

type productService struct {
	repo          repository.ProductRepository
	inventoryRepo repository.InventoryRepository
	auditLogRepo  repository.AuditLogRepository
	cache         cache.Cache
}

func NewProductService(repo repository.ProductRepository, inventoryRepo repository.InventoryRepository, auditLogRepo repository.AuditLogRepository, c cache.Cache) ProductService {
	return &productService{
		repo:          repo,
		inventoryRepo: inventoryRepo,
		auditLogRepo:  auditLogRepo,
		cache:         c,
	}
}

func (s *productService) invalidate(ctx context.Context, orgID uuid.UUID) {
	_ = s.cache.DeleteByPattern(ctx, cache.ProductsListPattern(orgID))
	_ = s.cache.DeleteByPattern(ctx, cache.MarketplaceSearchPattern())
}

func (s *productService) Create(ctx context.Context, product *domain.Product, ipAddress string) error {
	existing, err := s.repo.GetBySKU(ctx, product.SKU, product.OrgID)
	if err == nil && existing != nil {
		return domain.ErrConflict
	}
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return err
	}

	product.ID = uuid.New()
	now := time.Now().UTC()
	product.CreatedAt = now
	product.UpdatedAt = now

	if err := s.repo.Create(ctx, product); err != nil {
		return err
	}

	inv := &domain.Inventory{
		ID:                uuid.New(),
		OrgID:             product.OrgID,
		ProductID:         product.ID,
		Quantity:          0,
		LowStockThreshold: 10,
		UpdatedAt:         now,
	}
	if err := s.inventoryRepo.Create(ctx, inv); err != nil {
		zap.L().Error("failed to create inventory record", zap.Error(err))
	}

	audit := &domain.AuditLog{
		ID:        uuid.New(),
		OrgID:     product.OrgID,
		UserID:    nil,
		Action:    "product.created",
		Entity:    "products",
		EntityID:  product.ID,
		IPAddress: ipAddress,
		CreatedAt: now,
	}
	if err := s.auditLogRepo.Create(ctx, audit); err != nil {
		zap.L().Error("audit log failed", zap.Error(err))
	}

	s.invalidate(ctx, product.OrgID)
	return nil
}

func (s *productService) GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*domain.Product, error) {
	return s.repo.GetByID(ctx, id, orgID)
}

func (s *productService) GetBySKU(ctx context.Context, sku string, orgID uuid.UUID) (*domain.Product, error) {
	return s.repo.GetBySKU(ctx, sku, orgID)
}

func (s *productService) Update(ctx context.Context, product *domain.Product) error {
	product.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, product); err != nil {
		return err
	}
	s.invalidate(ctx, product.OrgID)
	return nil
}

func (s *productService) Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID, ipAddress string) error {
	if err := s.repo.Delete(ctx, id, orgID); err != nil {
		return err
	}

	audit := &domain.AuditLog{
		ID:        uuid.New(),
		OrgID:     orgID,
		UserID:    nil,
		Action:    "product.deleted",
		Entity:    "products",
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

func (s *productService) List(ctx context.Context, orgID uuid.UUID) ([]*domain.Product, error) {
	key := cache.ProductsListKey(orgID)
	var cached []*domain.Product
	if err := s.cache.Get(ctx, key, &cached); err == nil {
		return cached, nil
	}

	products, err := s.repo.List(ctx, orgID)
	if err != nil {
		return nil, err
	}
	_ = s.cache.Set(ctx, key, products, cache.TTLMedium)
	return products, nil
}

func (s *productService) ListByCategory(ctx context.Context, categoryID uuid.UUID, orgID uuid.UUID) ([]*domain.Product, error) {
	return s.repo.ListByCategory(ctx, categoryID, orgID)
}
