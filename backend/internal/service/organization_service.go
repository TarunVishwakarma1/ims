package service

import (
	"context"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/google/uuid"
)

type organizationService struct {
	orgRepo domain.OrganizationRepository
}

func NewOrganizationService(orgRepo domain.OrganizationRepository) domain.OrganizationService {
	return &organizationService{
		orgRepo: orgRepo,
	}
}

func (s *organizationService) Create(ctx context.Context, org *domain.Organization) error {
	org.ID = uuid.New()
	org.CreatedAt = time.Now()
	org.UpdatedAt = time.Now()
	org.IsActive = true
	if org.PlanType == "" {
		org.PlanType = "free"
	}
	return s.orgRepo.Create(ctx, org)
}

func (s *organizationService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Organization, error) {
	return s.orgRepo.GetByID(ctx, id)
}

func (s *organizationService) GetBySlug(ctx context.Context, slug string) (*domain.Organization, error) {
	return s.orgRepo.GetBySlug(ctx, slug)
}
