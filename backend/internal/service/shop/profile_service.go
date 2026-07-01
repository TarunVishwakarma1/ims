package shop

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
)

var (
	ErrInvalidProfileSlug = errors.New("slug must be lowercase letters, numbers, and single hyphens")
	ErrSlugTaken         = errors.New("that storefront address is already taken")
	ErrSlugLocked        = errors.New("storefront address can't change once the shop is live")
	ErrGoLiveIncomplete  = errors.New("to go live, set a display name, map location, and at least one pincode")
)

// slugRe matches DNS-safe slugs: lowercase alnum with single internal hyphens.
var slugRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type UpsertProfileInput struct {
	Slug        string
	DisplayName string
	Tagline     string
	LogoURL     string
	Area        string
	City        string
	Pincodes    []string
	Lat         *float64
	Lng         *float64
	IsLive      bool
}

type ShopProfileService interface {
	GetMine(ctx context.Context, orgID uuid.UUID) (*domain.ShopProfile, error)
	Upsert(ctx context.Context, orgID uuid.UUID, in UpsertProfileInput) (*domain.ShopProfile, error)
}

type shopProfileService struct{ repo domain.ShopProfileRepository }

func NewShopProfileService(repo domain.ShopProfileRepository) ShopProfileService {
	return &shopProfileService{repo: repo}
}

func (s *shopProfileService) GetMine(ctx context.Context, orgID uuid.UUID) (*domain.ShopProfile, error) {
	return s.repo.GetByOrg(ctx, orgID)
}

func (s *shopProfileService) Upsert(ctx context.Context, orgID uuid.UUID, in UpsertProfileInput) (*domain.ShopProfile, error) {
	slug := strings.ToLower(strings.TrimSpace(in.Slug))
	if !slugRe.MatchString(slug) {
		return nil, ErrInvalidProfileSlug
	}
	if strings.TrimSpace(in.DisplayName) == "" {
		return nil, ErrGoLiveIncomplete
	}
	if in.Lat != nil && (*in.Lat < -90 || *in.Lat > 90) {
		return nil, errors.New("latitude out of range")
	}
	if in.Lng != nil && (*in.Lng < -180 || *in.Lng > 180) {
		return nil, errors.New("longitude out of range")
	}

	// Load any existing profile — needed for slug-lock.
	existing, err := s.repo.GetByOrg(ctx, orgID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	if existing != nil && existing.IsLive && slug != existing.Slug {
		return nil, ErrSlugLocked
	}

	// Slug uniqueness across other orgs.
	taken, err := s.repo.SlugTakenByOther(ctx, slug, orgID)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, ErrSlugTaken
	}

	// Go-live completeness guard.
	if in.IsLive {
		if in.Lat == nil || in.Lng == nil || len(in.Pincodes) == 0 {
			return nil, ErrGoLiveIncomplete
		}
	}

	p := &domain.ShopProfile{
		OrgID: orgID, Slug: slug, DisplayName: strings.TrimSpace(in.DisplayName),
		Tagline: in.Tagline, LogoURL: in.LogoURL, Area: in.Area, City: in.City,
		Pincodes: in.Pincodes, Lat: in.Lat, Lng: in.Lng, IsLive: in.IsLive,
	}
	if p.Pincodes == nil {
		p.Pincodes = []string{}
	}
	if err := s.repo.Upsert(ctx, p); err != nil {
		return nil, err
	}
	return s.repo.GetByOrg(ctx, orgID)
}
