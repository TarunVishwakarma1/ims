package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"strings"
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

type fakeStore struct{ savedKey string }

func (f *fakeStore) Save(_ context.Context, key string, _ io.Reader, _ string) (string, error) {
	f.savedKey = key
	return "/uploads/" + key, nil
}
func (f *fakeStore) Delete(_ context.Context, _ string) error { return nil }

func TestAdminBanner_Create_201(t *testing.T) {
	now := time.Now()
	h := handler.NewAdminBannerHandler(&fakeAdminSvc{createOut: &domain.Banner{ID: uuid.New(), Title: "X"}}, &fakeStore{}, 1<<20)
	body, _ := json.Marshal(srv.BannerInput{Title: "X", StartsAt: now, EndsAt: now.Add(time.Hour)})
	rec := httptest.NewRecorder()
	h.Create(rec, httptest.NewRequest("POST", "/banners", bytes.NewReader(body)))
	if rec.Code != 201 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminBanner_Publish_409_HeroConflict(t *testing.T) {
	h := handler.NewAdminBannerHandler(&fakeAdminSvc{publishErr: srv.ErrHeroConflict}, &fakeStore{}, 1<<20)
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
	h := handler.NewAdminBannerHandler(&fakeAdminSvc{getErr: srv.ErrBannerNotFound}, &fakeStore{}, 1<<20)
	r := chi.NewRouter()
	r.Get("/banners/{id}", h.Get)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/banners/"+uuid.New().String(), nil))
	if rec.Code != 404 {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestAdminBanner_Upload_200(t *testing.T) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "x.jpg")
	_, _ = fw.Write([]byte{0xff, 0xd8, 0xff, 0xe0}) // JPEG magic
	_ = mw.Close()

	store := &fakeStore{}
	h := handler.NewAdminBannerHandler(&fakeAdminSvc{}, store, 1<<20)
	req := httptest.NewRequest("POST", "/banners/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.Upload(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "/uploads/banners/") {
		t.Fatalf("missing url: %s", rec.Body.String())
	}
}

func TestAdminBanner_Upload_TooLarge_400(t *testing.T) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "x.jpg")
	big := make([]byte, 2<<20) // 2 MB
	big[0], big[1], big[2], big[3] = 0xff, 0xd8, 0xff, 0xe0
	_, _ = fw.Write(big)
	_ = mw.Close()

	h := handler.NewAdminBannerHandler(&fakeAdminSvc{}, &fakeStore{}, 1<<20) // 1 MB cap
	req := httptest.NewRequest("POST", "/banners/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.Upload(rec, req)
	if rec.Code != 400 {
		t.Fatalf("code=%d", rec.Code)
	}
}
