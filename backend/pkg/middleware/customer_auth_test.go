package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
)

func TestRequireCustomer_HappyPath(t *testing.T) {
	cid := uuid.New()
	tok, _ := shop.IssueShopJWT("s", cid, time.Hour)
	mw := middleware.RequireCustomer("s")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := middleware.GetCustomerIDFromContext(r.Context())
		if !ok || got != cid {
			t.Fatalf("ctx missing or wrong: %v %v", got, ok)
		}
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("expected 200 got %d", rec.Code)
	}
}

func TestRequireCustomer_RejectsMissing(t *testing.T) {
	mw := middleware.RequireCustomer("s")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be reached")
	}))
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 401 {
		t.Fatalf("expected 401 got %d", rec.Code)
	}
}
