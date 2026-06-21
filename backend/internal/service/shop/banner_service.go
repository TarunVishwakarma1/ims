package shop

import (
	"context"
	"errors"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"

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
	ErrInvalidSlug      = errors.New("category_slug must match [a-z0-9-]{1,200}")
	ErrInvalidEventKey  = errors.New("event_key must match [a-z0-9_-]{1,200}")
)

var (
	bannerSlugRe     = regexp.MustCompile(`^[a-z0-9-]{1,200}$`)
	bannerEventKeyRe = regexp.MustCompile(`^[a-z0-9_-]{1,200}$`)
)

type ActiveBanners struct {
	Hero     *domain.Banner  `json:"hero,omitempty"`
	Carousel []domain.Banner `json:"carousel"`
}

type BannerInput struct {
	Title          string    `json:"title"`
	Subtitle       string    `json:"subtitle,omitempty"`
	ImageURL       string    `json:"image_url,omitempty"`
	CTALabel       string    `json:"cta_label,omitempty"`
	CTALink        string    `json:"cta_link,omitempty"`
	EventKey       string    `json:"event_key,omitempty"`
	StartsAt       time.Time `json:"starts_at"`
	EndsAt         time.Time `json:"ends_at"`
	SortOrder      int       `json:"sort_order,omitempty"`
	IsHero         bool      `json:"is_hero,omitempty"`
	AudienceFilter string    `json:"audience_filter,omitempty"`
	CategorySlug   string    `json:"category_slug,omitempty"`
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

var validAudiences = map[string]struct{}{"": {}, "all": {}, "new": {}, "returning": {}}

func (s *bannerService) Create(ctx context.Context, in BannerInput) (*domain.Banner, error) {
	if err := validateInput(in); err != nil {
		return nil, err
	}
	b := &domain.Banner{
		OrgID:          s.orgID,
		Title:          in.Title,
		Subtitle:       in.Subtitle,
		ImageURL:       in.ImageURL,
		CTALabel:       in.CTALabel,
		CTALink:        in.CTALink,
		EventKey:       in.EventKey,
		StartsAt:       in.StartsAt,
		EndsAt:         in.EndsAt,
		Status:         "draft",
		SortOrder:      in.SortOrder,
		IsHero:         in.IsHero,
		AudienceFilter: in.AudienceFilter,
		CategorySlug:   in.CategorySlug,
	}
	out, err := s.repo.Insert(ctx, b)
	if err != nil {
		return nil, err
	}
	if err := s.InvalidateActive(ctx); err != nil {
		zap.L().Warn("banner cache invalidate failed", zap.Error(err))
	}
	return out, nil
}

func (s *bannerService) Update(ctx context.Context, id uuid.UUID, in BannerInput) (*domain.Banner, error) {
	if err := validateInput(in); err != nil {
		return nil, err
	}
	current, err := s.repo.GetByID(ctx, s.orgID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, ErrBannerNotFound
		}
		return nil, err
	}
	current.Title = in.Title
	current.Subtitle = in.Subtitle
	current.ImageURL = in.ImageURL
	current.CTALabel = in.CTALabel
	current.CTALink = in.CTALink
	current.EventKey = in.EventKey
	current.StartsAt = in.StartsAt
	current.EndsAt = in.EndsAt
	current.SortOrder = in.SortOrder
	current.IsHero = in.IsHero
	current.AudienceFilter = in.AudienceFilter
	current.CategorySlug = in.CategorySlug
	out, err := s.repo.Update(ctx, current)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, ErrBannerNotFound
		}
		return nil, err
	}
	if err := s.InvalidateActive(ctx); err != nil {
		zap.L().Warn("banner cache invalidate failed", zap.Error(err))
	}
	return out, nil
}

func (s *bannerService) Publish(ctx context.Context, id uuid.UUID) error {
	current, err := s.repo.GetByID(ctx, s.orgID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return ErrBannerNotFound
		}
		return err
	}
	if current.ImageURL == "" {
		return ErrImageRequired
	}
	if current.IsHero {
		has, err := s.repo.HasOtherPublishedHero(ctx, s.orgID, id)
		if err != nil {
			return err
		}
		if has {
			return ErrHeroConflict
		}
	}
	current.Status = "published"
	if _, err := s.repo.Update(ctx, current); err != nil {
		if isUniqueViolation(err) {
			return ErrHeroConflict
		}
		return err
	}
	if err := s.InvalidateActive(ctx); err != nil {
		zap.L().Warn("banner cache invalidate failed", zap.Error(err))
	}
	return nil
}

func (s *bannerService) Archive(ctx context.Context, id uuid.UUID) error {
	current, err := s.repo.GetByID(ctx, s.orgID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return ErrBannerNotFound
		}
		return err
	}
	current.Status = "archived"
	if _, err := s.repo.Update(ctx, current); err != nil {
		return err
	}
	if err := s.InvalidateActive(ctx); err != nil {
		zap.L().Warn("banner cache invalidate failed", zap.Error(err))
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

func (s *bannerService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, s.orgID, id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return ErrBannerNotFound
		}
		return err
	}
	if err := s.InvalidateActive(ctx); err != nil {
		zap.L().Warn("banner cache invalidate failed", zap.Error(err))
	}
	return nil
}

func (s *bannerService) Get(ctx context.Context, id uuid.UUID) (*domain.Banner, error) {
	b, err := s.repo.GetByID(ctx, s.orgID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, ErrBannerNotFound
		}
		return nil, err
	}
	return b, nil
}

func (s *bannerService) List(ctx context.Context, q BannerListQuery) ([]domain.Banner, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 24
	}
	if limit > 100 {
		limit = 100
	}
	if q.Status != "" && !validStatus(q.Status) {
		return nil, ErrInvalidStatus
	}
	return s.repo.List(ctx, s.orgID, q.Status, q.EventKey, limit, q.Offset)
}

func validateInput(in BannerInput) error {
	if !in.EndsAt.After(in.StartsAt) {
		return ErrInvalidDateRange
	}
	if _, ok := validAudiences[in.AudienceFilter]; !ok {
		return ErrInvalidAudience
	}
	if in.CategorySlug != "" && !bannerSlugRe.MatchString(in.CategorySlug) {
		return ErrInvalidSlug
	}
	if in.EventKey != "" && !bannerEventKeyRe.MatchString(in.EventKey) {
		return ErrInvalidEventKey
	}
	return nil
}

func validStatus(s string) bool {
	switch s {
	case "draft", "published", "archived":
		return true
	}
	return false
}
