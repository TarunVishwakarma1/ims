package service

import (
	"context"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/pkg/cache"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type CategoryService interface {
	Create(ctx context.Context, category *domain.Category, ipAddress string) error
	GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*domain.Category, error)
	Update(ctx context.Context, category *domain.Category) error
	Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID, ipAddress string) error
	List(ctx context.Context, orgID uuid.UUID) ([]*domain.Category, error)
}

type categoryService struct {
	repo         repository.CategoryRepository
	auditLogRepo repository.AuditLogRepository
	cache        cache.Cache
}

func NewCategoryService(repo repository.CategoryRepository, auditLogRepo repository.AuditLogRepository, c cache.Cache) CategoryService {
	return &categoryService{
		repo:         repo,
		auditLogRepo: auditLogRepo,
		cache:        c,
	}
}

func (s *categoryService) invalidate(ctx context.Context, orgID uuid.UUID) {
	_ = s.cache.DeleteByPattern(ctx, cache.CategoriesListPattern(orgID))
}

func (s *categoryService) Create(ctx context.Context, category *domain.Category, ipAddress string) error {
	category.ID = uuid.New()
	now := time.Now().UTC()
	category.CreatedAt = now
	category.UpdatedAt = now

	if err := s.repo.Create(ctx, category); err != nil {
		return err
	}

	audit := &domain.AuditLog{
		ID:        uuid.New(),
		OrgID:     category.OrgID,
		UserID:    nil,
		Action:    "category.created",
		Entity:    "categories",
		EntityID:  category.ID,
		IPAddress: ipAddress,
		CreatedAt: now,
	}
	if err := s.auditLogRepo.Create(ctx, audit); err != nil {
		zap.L().Error("audit log failed", zap.Error(err))
	}

	s.invalidate(ctx, category.OrgID)
	return nil
}

func (s *categoryService) GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*domain.Category, error) {
	return s.repo.GetByID(ctx, id, orgID)
}

func (s *categoryService) Update(ctx context.Context, category *domain.Category) error {
	category.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, category); err != nil {
		return err
	}
	s.invalidate(ctx, category.OrgID)
	return nil
}

func (s *categoryService) Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID, ipAddress string) error {
	if err := s.repo.Delete(ctx, id, orgID); err != nil {
		return err
	}

	audit := &domain.AuditLog{
		ID:        uuid.New(),
		OrgID:     orgID,
		UserID:    nil,
		Action:    "category.deleted",
		Entity:    "categories",
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

func (s *categoryService) List(ctx context.Context, orgID uuid.UUID) ([]*domain.Category, error) {
	key := cache.CategoriesListKey(orgID)
	var cached []*domain.Category
	if err := s.cache.Get(ctx, key, &cached); err == nil {
		return cached, nil
	}

	cats, err := s.repo.List(ctx, orgID)
	if err != nil {
		return nil, err
	}
	_ = s.cache.Set(ctx, key, cats, cache.TTLLong)
	return cats, nil
}
