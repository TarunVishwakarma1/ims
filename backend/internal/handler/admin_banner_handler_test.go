package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/handler"
	srv "github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
)

type fakeAdminSvc struct {
	createOut  *domain.Banner
	createErr  error
	publishErr error
	getOut     *domain.Banner
	getErr     error
}

func (f *fakeAdminSvc) ListActive(_ context.Context, _ string) (*srv.ActiveBanners, error) { return nil, nil }
func (f *fakeAdminSvc) InvalidateActive(_ context.Context) error                            { return nil }
func (f *fakeAdminSvc) Create(_ context.Context, _ srv.BannerInput) (*domain.Banner, error) {
	return f.createOut, f.createErr
}
func (f *fakeAdminSvc) Update(_ context.Context, _ uuid.UUID, _ srv.BannerInput) (*domain.Banner, error) {
	return nil, nil
}
func (f *fakeAdminSvc) Publish(_ context.Context, _ uuid.UUID) error { return f.publishErr }
func (f *fakeAdminSvc) Archive(_ context.Context, _ uuid.UUID) error  { return nil }
func (f *fakeAdminSvc) Delete(_ context.Context, _ uuid.UUID) error   { return nil }
func (f *fakeAdminSvc) Get(_ context.Context, _ uuid.UUID) (*domain.Banner, error) {
	return f.getOut, f.getErr
}
func (f *fakeAdminSvc) List(_ context.Context, _ srv.BannerListQuery) ([]domain.Banner, error) {
	return nil, nil
}

func TestAdminBanner_Create_201(t *testing.T) {
	now := time.Now()
	h := handler.NewAdminBannerHandler(&fakeAdminSvc{createOut: &domain.Banner{ID: uuid.New(), Title: "X"}})
	body, _ := json.Marshal(srv.BannerInput{Title: "X", StartsAt: now, EndsAt: now.Add(time.Hour)})
	rec := httptest.NewRecorder()
	h.Create(rec, httptest.NewRequest("POST", "/banners", bytes.NewReader(body)))
	if rec.Code != 201 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminBanner_Publish_409_HeroConflict(t *testing.T) {
	h := handler.NewAdminBannerHandler(&fakeAdminSvc{publishErr: srv.ErrHeroConflict})
	r := chi.NewRouter()
	r.Post("/banners/{id}/publish", h.Publish)
	id := uuid.New().String()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("POST", "/banners/"+id+"/publish", nil))
	if rec.Code != 409 {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestAdminBanner_Get_404(t *testing.T) {
	h := handler.NewAdminBannerHandler(&fakeAdminSvc{getErr: srv.ErrBannerNotFound})
	r := chi.NewRouter()
	r.Get("/banners/{id}", h.Get)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/banners/"+uuid.New().String(), nil))
	if rec.Code != 404 {
		t.Fatalf("code=%d", rec.Code)
	}
}
