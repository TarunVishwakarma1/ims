package shop_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	shophandler "github.com/TarunVishwakarma1/ims/backend/internal/handler/shop"
	srv "github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
)

// fakeCatalog implements srv.CatalogService for handler tests.
type fakeCatalog struct {
	cats      []srv.CategoryView
	listRes   *srv.ProductListResult
	listErr   error
	detail    *srv.ProductDetail
	detailErr error
}

func (f *fakeCatalog) ListCategories(_ context.Context) ([]srv.CategoryView, error) { return f.cats, nil }
func (f *fakeCatalog) ListProducts(_ context.Context, _ srv.ProductListQuery) (*srv.ProductListResult, error) {
	return f.listRes, f.listErr
}
func (f *fakeCatalog) GetProductBySlug(_ context.Context, _ string) (*srv.ProductDetail, error) {
	return f.detail, f.detailErr
}
func (f *fakeCatalog) InvalidateCategories(_ context.Context) error        { return nil }
func (f *fakeCatalog) InvalidateProductList(_ context.Context) error       { return nil }
func (f *fakeCatalog) InvalidateProduct(_ context.Context, _ string) error { return nil }

// fakeFeed implements srv.FeedService for handler tests.
type fakeFeed struct {
	pg  *srv.FeedPage
	err error
}

func (f *fakeFeed) Page(_ context.Context, _ string, _ string, _ int) (*srv.FeedPage, error) {
	return f.pg, f.err
}

func TestCatalogHandler_ListCategories_200(t *testing.T) {
	f := &fakeCatalog{cats: []srv.CategoryView{{Name: "Snacks", Slug: "snacks"}}}
	h := shophandler.NewCatalogHandler(f, nil)
	rec := httptest.NewRecorder()
	h.ListCategories(rec, httptest.NewRequest("GET", "/categories", nil))
	if rec.Code != 200 {
		t.Fatalf("code=%d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Cache-Control"), "max-age=60") {
		t.Fatalf("missing Cache-Control: %q", rec.Header().Get("Cache-Control"))
	}
	if rec.Header().Get("Server-Timing") == "" {
		t.Fatal("missing Server-Timing")
	}
}

func TestCatalogHandler_ListProducts_InvalidSort_400(t *testing.T) {
	h := shophandler.NewCatalogHandler(&fakeCatalog{listErr: errors.New("invalid_sort")}, nil)
	rec := httptest.NewRecorder()
	h.ListProducts(rec, httptest.NewRequest("GET", "/products?sort=lolwut", nil))
	if rec.Code != 400 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "invalid_sort") {
		t.Fatalf("body: %s", rec.Body.String())
	}
}

func TestCatalogHandler_GetProductBySlug_NotFound_404(t *testing.T) {
	h := shophandler.NewCatalogHandler(&fakeCatalog{detailErr: srv.ErrNotFound}, nil)
	r := chi.NewRouter()
	r.Get("/products/{slug}", h.GetProductBySlug)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/products/no-such-slug", nil))
	if rec.Code != 404 {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestCatalogHandler_GetProductBySlug_IfNoneMatch_304(t *testing.T) {
	d := &srv.ProductDetail{ProductCard: srv.ProductCard{Slug: "p", Name: "P"}}
	h := shophandler.NewCatalogHandler(&fakeCatalog{detail: d}, nil)
	r := chi.NewRouter()
	r.Get("/products/{slug}", h.GetProductBySlug)

	first := httptest.NewRecorder()
	r.ServeHTTP(first, httptest.NewRequest("GET", "/products/p", nil))
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("missing ETag on first response")
	}

	req := httptest.NewRequest("GET", "/products/p", nil)
	req.Header.Set("If-None-Match", etag)
	second := httptest.NewRecorder()
	r.ServeHTTP(second, req)
	if second.Code != 304 {
		t.Fatalf("expected 304, got %d", second.Code)
	}
}

func TestCatalogHandler_GetProductBySlug_InvalidSlug_400(t *testing.T) {
	h := shophandler.NewCatalogHandler(&fakeCatalog{}, nil)
	r := chi.NewRouter()
	r.Get("/products/{slug}", h.GetProductBySlug)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/products/<script>", nil))
	if rec.Code != 400 {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestCatalogHandler_Feed_200(t *testing.T) {
	pg := &srv.FeedPage{Items: []srv.ProductCard{{Slug: "a"}}, NextCursor: "next", PageInfo: srv.FeedPageInfo{Tier: "category", Page: 1}}
	h := shophandler.NewCatalogHandler(&fakeCatalog{}, &fakeFeed{pg: pg})
	rec := httptest.NewRecorder()
	h.Feed(rec, httptest.NewRequest("GET", "/feed?seed_category=snacks&limit=24", nil))
	if rec.Code != 200 {
		t.Fatalf("code=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"tier":"category"`) {
		t.Fatalf("body: %s", rec.Body.String())
	}
}
