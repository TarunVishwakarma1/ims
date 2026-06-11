package service

import (
	"context"
	"fmt"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/google/uuid"
)

type CategoryService interface {
	Create(ctx context.Context, category *domain.Category, ipAddress string) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Category, error)
	Update(ctx context.Context, category *domain.Category) error
	Delete(ctx context.Context, id uuid.UUID, ipAddress string) error
	List(ctx context.Context) ([]*domain.Category, error)
}

type categoryService struct {
	repo         repository.CategoryRepository
	auditLogRepo repository.AuditLogRepository
}

func NewCategoryService(repo repository.CategoryRepository, auditLogRepo repository.AuditLogRepository) CategoryService {
	return &categoryService{
		repo:         repo,
		auditLogRepo: auditLogRepo,
	}
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
		UserID:    nil,
		Action:    "category.created",
		Entity:    "categories",
		EntityID:  category.ID,
		IPAddress: ipAddress,
		CreatedAt: now,
	}
	if err := s.auditLogRepo.Create(ctx, audit); err != nil {
		fmt.Printf("audit log failed: %v\n", err)
	}

	return nil
}

func (s *categoryService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Category, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *categoryService) Update(ctx context.Context, category *domain.Category) error {
	category.UpdatedAt = time.Now().UTC()
	return s.repo.Update(ctx, category)
}

func (s *categoryService) Delete(ctx context.Context, id uuid.UUID, ipAddress string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	audit := &domain.AuditLog{
		ID:        uuid.New(),
		UserID:    nil,
		Action:    "category.deleted",
		Entity:    "categories",
		EntityID:  id,
		IPAddress: ipAddress,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.auditLogRepo.Create(ctx, audit); err != nil {
		fmt.Printf("audit log failed: %v\n", err)
	}

	return nil
}

func (s *categoryService) List(ctx context.Context) ([]*domain.Category, error) {
	return s.repo.List(ctx)
}
