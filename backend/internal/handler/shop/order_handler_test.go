package shop_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	shophandler "github.com/TarunVishwakarma1/ims/backend/internal/handler/shop"
	srv "github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
)

type fakeShopOrders struct {
	list      *srv.OrderListResult
	listErr   error
	detail    *srv.OrderDetail
	detailErr error
	cancel    *srv.CancelResult
	cancelErr error
}

func (f *fakeShopOrders) List(_ context.Context, _ uuid.UUID, _ srv.OrderListQuery) (*srv.OrderListResult, error) {
	return f.list, f.listErr
}
func (f *fakeShopOrders) Get(_ context.Context, _, _ uuid.UUID) (*srv.OrderDetail, error) {
	return f.detail, f.detailErr
}
func (f *fakeShopOrders) Cancel(_ context.Context, _, _ uuid.UUID) (*srv.CancelResult, error) {
	return f.cancel, f.cancelErr
}

func withCustomerCtx(req *http.Request, customerID uuid.UUID) *http.Request {
	ctx := context.WithValue(req.Context(), middleware.ContextKeyCustomerID, customerID)
	return req.WithContext(ctx)
}

func TestOrderHandler_List_200(t *testing.T) {
	h := shophandler.NewOrderHandler(&fakeShopOrders{
		list: &srv.OrderListResult{Items: []srv.OrderCard{{InvoiceNumber: "INV1"}}},
	})
	customerID := uuid.New()
	r := chi.NewRouter()
	r.Get("/orders", h.List)

	req := httptest.NewRequest("GET", "/orders", nil)
	req = withCustomerCtx(req, customerID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("code=%d", rec.Code)
	}
	if rec.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("Cache-Control: %s", rec.Header().Get("Cache-Control"))
	}
}

func TestOrderHandler_Get_404(t *testing.T) {
	h := shophandler.NewOrderHandler(&fakeShopOrders{detailErr: srv.ErrOrderNotFound})
	customerID := uuid.New()
	r := chi.NewRouter()
	r.Get("/orders/{id}", h.Get)

	req := httptest.NewRequest("GET", "/orders/"+uuid.New().String(), nil)
	req = withCustomerCtx(req, customerID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 404 {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestOrderHandler_Cancel_409_ConflictState(t *testing.T) {
	h := shophandler.NewOrderHandler(&fakeShopOrders{cancelErr: srv.ErrCancelNotAllowed})
	customerID := uuid.New()
	r := chi.NewRouter()
	r.Post("/orders/{id}/cancel", h.Cancel)

	req := httptest.NewRequest("POST", "/orders/"+uuid.New().String()+"/cancel", nil)
	req = withCustomerCtx(req, customerID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 409 {
		t.Fatalf("code=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "conflict_state") {
		t.Fatalf("body: %s", rec.Body.String())
	}
}
