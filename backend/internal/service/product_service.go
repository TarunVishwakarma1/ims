package service

import (
	"context"
	"errors"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type ProductService interface {
	Create(ctx context.Context, product *domain.Product, ipAddress string) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Product, error)
	GetBySKU(ctx context.Context, sku string) (*domain.Product, error)
	Update(ctx context.Context, product *domain.Product) error
	Delete(ctx context.Context, id uuid.UUID, ipAddress string) error
	List(ctx context.Context) ([]*domain.Product, error)
	ListByCategory(ctx context.Context, categoryID uuid.UUID) ([]*domain.Product, error)
}

type productService struct {
	repo         repository.ProductRepository
	auditLogRepo repository.AuditLogRepository
}

func NewProductService(repo repository.ProductRepository, auditLogRepo repository.AuditLogRepository) ProductService {
	return &productService{
		repo:         repo,
		auditLogRepo: auditLogRepo,
	}
}

func (s *productService) Create(ctx context.Context, product *domain.Product, ipAddress string) error {
	existing, err := s.repo.GetBySKU(ctx, product.SKU)
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

	audit := &domain.AuditLog{
		ID:        uuid.New(),
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

	return nil
}

func (s *productService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Product, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *productService) GetBySKU(ctx context.Context, sku string) (*domain.Product, error) {
	return s.repo.GetBySKU(ctx, sku)
}

func (s *productService) Update(ctx context.Context, product *domain.Product) error {
	product.UpdatedAt = time.Now().UTC()
	return s.repo.Update(ctx, product)
}

func (s *productService) Delete(ctx context.Context, id uuid.UUID, ipAddress string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	audit := &domain.AuditLog{
		ID:        uuid.New(),
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

	return nil
}

func (s *productService) List(ctx context.Context) ([]*domain.Product, error) {
	return s.repo.List(ctx)
}

func (s *productService) ListByCategory(ctx context.Context, categoryID uuid.UUID) ([]*domain.Product, error) {
	return s.repo.ListByCategory(ctx, categoryID)
}
