package service

import (
	"context"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/pkg/cache"
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
	repo  repository.LocationRepository
	cache cache.Cache
}

func NewLocationService(repo repository.LocationRepository, c cache.Cache) LocationService {
	return &locationService{repo: repo, cache: c}
}

func (s *locationService) invalidate(ctx context.Context, orgID uuid.UUID) {
	_ = s.cache.DeleteByPattern(ctx, cache.LocationsListPattern(orgID))
	// Listings reference locations; bust marketplace search too.
	_ = s.cache.DeleteByPattern(ctx, cache.MarketplaceSearchPattern())
}

func (s *locationService) Create(ctx context.Context, location *domain.OrgLocation) error {
	location.ID = uuid.New()
	location.IsActive = true
	now := time.Now()
	location.CreatedAt = now
	location.UpdatedAt = now
	if err := s.repo.Create(ctx, location); err != nil {
		return err
	}
	s.invalidate(ctx, location.OrgID)
	return nil
}

func (s *locationService) GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*domain.OrgLocation, error) {
	return s.repo.GetByID(ctx, id, orgID)
}

func (s *locationService) List(ctx context.Context, orgID uuid.UUID) ([]*domain.OrgLocation, error) {
	key := cache.LocationsListKey(orgID)
	var cached []*domain.OrgLocation
	if err := s.cache.Get(ctx, key, &cached); err == nil {
		return cached, nil
	}

	list, err := s.repo.List(ctx, orgID)
	if err != nil {
		return nil, err
	}
	_ = s.cache.Set(ctx, key, list, cache.TTLLong)
	return list, nil
}

func (s *locationService) Update(ctx context.Context, location *domain.OrgLocation) error {
	location.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, location); err != nil {
		return err
	}
	s.invalidate(ctx, location.OrgID)
	return nil
}

func (s *locationService) Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error {
	if err := s.repo.Delete(ctx, id, orgID); err != nil {
		return err
	}
	s.invalidate(ctx, orgID)
	return nil
}
