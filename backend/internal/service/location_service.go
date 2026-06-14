package service

import (
	"context"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/google/uuid"
)

type LocationService interface {
	Create(ctx context.Context, location *domain.OrgLocation) error
	GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*domain.OrgLocation, error)
	List(ctx context.Context, orgID uuid.UUID) ([]*domain.OrgLocation, error)
	Update(ctx context.Context, location *domain.OrgLocation) error
	Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error
}

type locationService struct {
	repo repository.LocationRepository
}

func NewLocationService(repo repository.LocationRepository) LocationService {
	return &locationService{repo: repo}
}

func (s *locationService) Create(ctx context.Context, location *domain.OrgLocation) error {
	location.ID = uuid.New()
	now := time.Now()
	location.CreatedAt = now
	location.UpdatedAt = now
	return s.repo.Create(ctx, location)
}

func (s *locationService) GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*domain.OrgLocation, error) {
	return s.repo.GetByID(ctx, id, orgID)
}

func (s *locationService) List(ctx context.Context, orgID uuid.UUID) ([]*domain.OrgLocation, error) {
	return s.repo.List(ctx, orgID)
}

func (s *locationService) Update(ctx context.Context, location *domain.OrgLocation) error {
	location.UpdatedAt = time.Now()
	return s.repo.Update(ctx, location)
}

func (s *locationService) Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error {
	return s.repo.Delete(ctx, id, orgID)
}
