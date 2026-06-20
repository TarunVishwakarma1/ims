package shop

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

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

var validAudiences = map[string]struct{}{"": {}, "all": {}, "new": {}, "returning": {}}

func (s *bannerService) Create(ctx context.Context, in BannerInput) (*domain.Banner, error) {
	if err := validateInput(in); err != nil { return nil, err }
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
		AudienceFilter: normalizeAudience(in.AudienceFilter),
		CategorySlug:   in.CategorySlug,
	}
	out, err := s.repo.Insert(ctx, b)
	if err != nil { return nil, err }
	_ = s.InvalidateActive(ctx)
	return out, nil
}

func (s *bannerService) Update(ctx context.Context, id uuid.UUID, in BannerInput) (*domain.Banner, error) {
	if err := validateInput(in); err != nil { return nil, err }
	current, err := s.repo.GetByID(ctx, s.orgID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) { return nil, ErrBannerNotFound }
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
	current.AudienceFilter = normalizeAudience(in.AudienceFilter)
	current.CategorySlug = in.CategorySlug
	out, err := s.repo.Update(ctx, current)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) { return nil, ErrBannerNotFound }
		return nil, err
	}
	_ = s.InvalidateActive(ctx)
	return out, nil
}

func (s *bannerService) Publish(ctx context.Context, id uuid.UUID) error {
	current, err := s.repo.GetByID(ctx, s.orgID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) { return ErrBannerNotFound }
		return err
	}
	if current.ImageURL == "" { return ErrImageRequired }
	if current.IsHero {
		// Defensive pre-check before relying on DB unique index.
		others, err := s.repo.List(ctx, s.orgID, "published", "", 100, 0)
		if err != nil { return err }
		for _, o := range others {
			if o.IsHero && o.ID != id { return ErrHeroConflict }
		}
	}
	current.Status = "published"
	if _, err := s.repo.Update(ctx, current); err != nil {
		if isUniqueViolation(err) { return ErrHeroConflict }
		return err
	}
	_ = s.InvalidateActive(ctx)
	return nil
}

func (s *bannerService) Archive(ctx context.Context, id uuid.UUID) error {
	current, err := s.repo.GetByID(ctx, s.orgID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) { return ErrBannerNotFound }
		return err
	}
	current.Status = "archived"
	if _, err := s.repo.Update(ctx, current); err != nil { return err }
	_ = s.InvalidateActive(ctx)
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
		if errors.Is(err, domain.ErrNotFound) { return ErrBannerNotFound }
		return err
	}
	_ = s.InvalidateActive(ctx)
	return nil
}

func (s *bannerService) Get(ctx context.Context, id uuid.UUID) (*domain.Banner, error) {
	b, err := s.repo.GetByID(ctx, s.orgID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) { return nil, ErrBannerNotFound }
		return nil, err
	}
	return b, nil
}

func (s *bannerService) List(ctx context.Context, q BannerListQuery) ([]domain.Banner, error) {
	limit := q.Limit
	if limit <= 0 { limit = 24 }
	if limit > 100 { limit = 100 }
	if q.Status != "" && !validStatus(q.Status) { return nil, ErrInvalidStatus }
	return s.repo.List(ctx, s.orgID, q.Status, q.EventKey, limit, q.Offset)
}

func validateInput(in BannerInput) error {
	if !in.EndsAt.After(in.StartsAt) { return ErrInvalidDateRange }
	if _, ok := validAudiences[in.AudienceFilter]; !ok { return ErrInvalidAudience }
	return nil
}

func normalizeAudience(a string) string { if a == "" { return "all" }; return a }

func validStatus(s string) bool {
	switch s {
	case "draft", "published", "archived":
		return true
	}
	return false
}
