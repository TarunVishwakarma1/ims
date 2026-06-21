package shop_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	shophandler "github.com/TarunVishwakarma1/ims/backend/internal/handler/shop"
	srv "github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
)

type fakeBanners struct {
	active *srv.ActiveBanners
	err    error
}

func (f *fakeBanners) ListActive(_ context.Context, _ string) (*srv.ActiveBanners, error) { return f.active, f.err }
func (f *fakeBanners) InvalidateActive(_ context.Context) error { return nil }
func (f *fakeBanners) Create(_ context.Context, _ srv.BannerInput) (*domain.Banner, error) { return nil, nil }
func (f *fakeBanners) Update(_ context.Context, _ uuid.UUID, _ srv.BannerInput) (*domain.Banner, error) { return nil, nil }
func (f *fakeBanners) Publish(_ context.Context, _ uuid.UUID) error { return nil }
func (f *fakeBanners) Archive(_ context.Context, _ uuid.UUID) error { return nil }
func (f *fakeBanners) Delete(_ context.Context, _ uuid.UUID) error  { return nil }
func (f *fakeBanners) Get(_ context.Context, _ uuid.UUID) (*domain.Banner, error) { return nil, nil }
func (f *fakeBanners) List(_ context.Context, _ srv.BannerListQuery) ([]domain.Banner, error) { return nil, nil }

func TestBannerHandler_ListActive_200_WithETag(t *testing.T) {
	h := shophandler.NewBannerHandler(&fakeBanners{
		active: &srv.ActiveBanners{
			Hero:     &domain.Banner{Title: "Hero"},
			Carousel: []domain.Banner{{Title: "Side"}},
		},
	})
	rec := httptest.NewRecorder()
	h.ListActive(rec, httptest.NewRequest("GET", "/banners/active", nil))
	if rec.Code != 200 { t.Fatalf("code=%d", rec.Code) }
	if !strings.Contains(rec.Body.String(), `"Hero"`) {
		t.Fatalf("body: %s", rec.Body.String())
	}
	if rec.Header().Get("ETag") == "" { t.Fatal("missing ETag") }
	if !strings.Contains(rec.Header().Get("Cache-Control"), "max-age=60") {
		t.Fatalf("Cache-Control: %s", rec.Header().Get("Cache-Control"))
	}
}

func TestBannerHandler_ListActive_IfNoneMatch_304(t *testing.T) {
	h := shophandler.NewBannerHandler(&fakeBanners{
		active: &srv.ActiveBanners{Hero: &domain.Banner{Title: "Hero"}},
	})
	first := httptest.NewRecorder()
	h.ListActive(first, httptest.NewRequest("GET", "/banners/active", nil))
	etag := first.Header().Get("ETag")

	req := httptest.NewRequest("GET", "/banners/active", nil)
	req.Header.Set("If-None-Match", etag)
	second := httptest.NewRecorder()
	h.ListActive(second, req)
	if second.Code != 304 {
		t.Fatalf("expected 304, got %d", second.Code)
	}
}
