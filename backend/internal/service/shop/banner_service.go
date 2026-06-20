package shop

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/pkg/cache"
)

var (
	ErrBannerNotFound   = errors.New("banner not found")
	ErrHeroConflict     = errors.New("another hero is published")
	ErrInvalidStatus    = errors.New("invalid status")
	ErrInvalidDateRange = errors.New("ends_at must be after starts_at")
	ErrImageRequired    = errors.New("image required to publish")
	ErrInvalidAudience  = errors.New("audience_filter must be one of all|new|returning")
)

type ActiveBanners struct {
	Hero     *domain.Banner  `json:"hero,omitempty"`
	Carousel []domain.Banner `json:"carousel"`
}

type BannerInput struct {
	Title, Subtitle, ImageURL, CTALabel, CTALink, EventKey string
	StartsAt, EndsAt                                       time.Time
	SortOrder                                              int
	IsHero                                                 bool
	AudienceFilter                                         string
	CategorySlug                                           string
}

type BannerListQuery struct {
	Status, EventKey string
	Limit, Offset    int
}

type BannerService interface {
	ListActive(ctx context.Context, categorySlug string) (*ActiveBanners, error)
	InvalidateActive(ctx context.Context) error
	Create(ctx context.Context, in BannerInput) (*domain.Banner, error)
	Update(ctx context.Context, id uuid.UUID, in BannerInput) (*domain.Banner, error)
	Publish(ctx context.Context, id uuid.UUID) error
	Archive(ctx context.Context, id uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (*domain.Banner, error)
	List(ctx context.Context, q BannerListQuery) ([]domain.Banner, error)
}

type bannerService struct {
	repo  repository.BannerRepository
	cache cache.Cache
	orgID uuid.UUID
}

func NewBannerService(repo repository.BannerRepository, c cache.Cache, orgID uuid.UUID) BannerService {
	return &bannerService{repo, c, orgID}
}

func (s *bannerService) ListActive(ctx context.Context, categorySlug string) (*ActiveBanners, error) {
	key := bannerActiveKey(s.orgID, categorySlug)
	var cached ActiveBanners
	if err := s.cache.Get(ctx, key, &cached); err == nil {
		return &cached, nil
	}
	rows, err := s.repo.ListActive(ctx, s.orgID, categorySlug, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	out := &ActiveBanners{Carousel: []domain.Banner{}}
	for i := range rows {
		if rows[i].IsHero && out.Hero == nil {
			b := rows[i]
			out.Hero = &b
			continue
		}
		out.Carousel = append(out.Carousel, rows[i])
	}
	_ = s.cache.Set(ctx, key, out, bannerCacheTTL)
	return out, nil
}

func (s *bannerService) InvalidateActive(ctx context.Context) error {
	return s.cache.DeleteByPattern(ctx, bannerCacheKeyPrefix+s.orgID.String()+"*")
}

// Stubs — Tasks 7/8 fill these.
func (s *bannerService) Create(_ context.Context, _ BannerInput) (*domain.Banner, error) {
	return nil, errors.New("not implemented")
}
func (s *bannerService) Update(_ context.Context, _ uuid.UUID, _ BannerInput) (*domain.Banner, error) {
	return nil, errors.New("not implemented")
}
func (s *bannerService) Publish(_ context.Context, _ uuid.UUID) error {
	return errors.New("not implemented")
}
func (s *bannerService) Archive(_ context.Context, _ uuid.UUID) error {
	return errors.New("not implemented")
}
func (s *bannerService) Delete(_ context.Context, _ uuid.UUID) error {
	return errors.New("not implemented")
}
func (s *bannerService) Get(_ context.Context, _ uuid.UUID) (*domain.Banner, error) {
	return nil, ErrBannerNotFound
}
func (s *bannerService) List(_ context.Context, _ BannerListQuery) ([]domain.Banner, error) {
	return nil, nil
}
