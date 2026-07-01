package shop_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	shophandler "github.com/TarunVishwakarma1/ims/backend/internal/handler/shop"
	shopsvc "github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
)

type fakeProfileSvc struct {
	got shopsvc.UpsertProfileInput
	err error
}

func (f *fakeProfileSvc) GetMine(ctx context.Context, orgID uuid.UUID) (*domain.ShopProfile, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeProfileSvc) Upsert(ctx context.Context, orgID uuid.UUID, in shopsvc.UpsertProfileInput) (*domain.ShopProfile, error) {
	f.got = in
	if f.err != nil {
		return nil, f.err
	}
	return &domain.ShopProfile{OrgID: orgID, Slug: in.Slug, DisplayName: in.DisplayName}, nil
}

// ctxWithOrg injects an org UUID into the context using the same key the
// middleware sets so that GetOrgIDFromContext can retrieve it in tests.
func ctxWithOrg(orgID uuid.UUID) context.Context {
	return context.WithValue(context.Background(), middleware.ContextKeyOrgID, orgID.String())
}

func TestProfileHandler_GetMine_404(t *testing.T) {
	h := shophandler.NewProfileHandler(&fakeProfileSvc{}, nil, 0)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/storefront", nil).
		WithContext(ctxWithOrg(uuid.New()))
	rr := httptest.NewRecorder()
	h.GetMine(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", rr.Code)
	}
	// Error responses must be JSON, not text/plain (regression: was http.Error).
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("want application/json error body, got %q", ct)
	}
}

func TestProfileHandler_Upsert_Conflict(t *testing.T) {
	h := shophandler.NewProfileHandler(&fakeProfileSvc{err: shopsvc.ErrSlugTaken}, nil, 0)
	body := `{"slug":"taken","display_name":"X"}`
	req := httptest.NewRequest(http.MethodPut, "/api/admin/storefront",
		strings.NewReader(body)).WithContext(ctxWithOrg(uuid.New()))
	rr := httptest.NewRecorder()
	h.Upsert(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d", rr.Code)
	}
}

func TestProfileHandler_Upsert_OK(t *testing.T) {
	svc := &fakeProfileSvc{}
	h := shophandler.NewProfileHandler(svc, nil, 0)
	body := `{"slug":"myshop","display_name":"My Shop","pincodes":["411001"],"lat":18.5,"lng":73.8,"is_live":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/admin/storefront",
		strings.NewReader(body)).WithContext(ctxWithOrg(uuid.New()))
	rr := httptest.NewRecorder()
	h.Upsert(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	var out domain.ShopProfile
	_ = json.NewDecoder(rr.Body).Decode(&out)
	if out.Slug != "myshop" || !svc.got.IsLive {
		t.Fatalf("unexpected: %+v / %+v", out, svc.got)
	}
}
