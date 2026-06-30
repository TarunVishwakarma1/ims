package shop

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	srv "github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
)

// fakeCart is a test double for srv.CartService
type fakeCart struct {
	items map[uuid.UUID]map[uuid.UUID]int // customer → product → qty
}

func newFakeCart() *fakeCart {
	return &fakeCart{
		items: make(map[uuid.UUID]map[uuid.UUID]int),
	}
}

func (f *fakeCart) Get(ctx context.Context, customerID uuid.UUID) (*srv.CartView, error) {
	v := &srv.CartView{Items: []srv.CartItemView{}}
	for pid, qty := range f.items[customerID] {
		v.Items = append(v.Items, srv.CartItemView{
			ProductID:      pid,
			Qty:            qty,
			UnitPricePaise: 100,
		})
		v.SubtotalPaise += int64(qty) * 100
	}
	return v, nil
}

func (f *fakeCart) AddOrSet(ctx context.Context, customerID, productID uuid.UUID, qty int, replace bool) (*srv.CartView, error) {
	if qty <= 0 {
		return nil, &validationError{"qty must be positive"}
	}
	if f.items[customerID] == nil {
		f.items[customerID] = make(map[uuid.UUID]int)
	}
	f.items[customerID][productID] = qty
	return f.Get(ctx, customerID)
}

func (f *fakeCart) Remove(ctx context.Context, customerID, productID uuid.UUID) (*srv.CartView, error) {
	if f.items[customerID] != nil {
		delete(f.items[customerID], productID)
	}
	return f.Get(ctx, customerID)
}

func (f *fakeCart) Clear(ctx context.Context, customerID uuid.UUID) error {
	f.items[customerID] = nil
	return nil
}

func (f *fakeCart) Merge(ctx context.Context, customerID uuid.UUID, items []srv.MergeItem) (*srv.CartView, error) {
	for _, it := range items {
		_, _ = f.AddOrSet(ctx, customerID, it.ProductID, it.Qty, false)
	}
	return f.Get(ctx, customerID)
}

// validationError is a simple error type
type validationError struct {
	msg string
}

func (e *validationError) Error() string {
	return e.msg
}

// Helper to build router with cart routes
func buildCartRouter(h *CartHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/cart", h.Get)
	r.Post("/cart/items", h.AddItem)
	r.Delete("/cart/items/{product_id}", h.RemoveItem)
	r.Post("/cart/merge", h.Merge)
	return r
}

// Helper to inject customerID into request context
func withCID(req *http.Request, cid uuid.UUID) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), middleware.ContextKeyCustomerID, cid))
}

// TestCartHandler_Get tests fetching an empty cart
func TestCartHandler_Get(t *testing.T) {
	customerID := uuid.New()
	fakeCart := newFakeCart()
	h := NewCartHandler(fakeCart)
	r := buildCartRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/cart", nil)
	req = withCID(req, customerID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var v srv.CartView
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if v.Items == nil || len(v.Items) != 0 {
		t.Errorf("expected empty items, got %+v", v.Items)
	}
}

// TestCartHandler_AddItem tests adding an item to the cart
func TestCartHandler_AddItem(t *testing.T) {
	customerID := uuid.New()
	productID := uuid.New()
	fakeCart := newFakeCart()
	h := NewCartHandler(fakeCart)
	r := buildCartRouter(h)

	body := map[string]interface{}{
		"product_id": productID,
		"qty":        5,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/cart/items", bytes.NewReader(bodyBytes))
	req = withCID(req, customerID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var v srv.CartView
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(v.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(v.Items))
	}
	if v.Items[0].ProductID != productID {
		t.Errorf("expected product_id %s, got %s", productID, v.Items[0].ProductID)
	}
	if v.Items[0].Qty != 5 {
		t.Errorf("expected qty 5, got %d", v.Items[0].Qty)
	}
}

// TestCartHandler_AddItem_ZeroQty tests adding an item with qty=0 returns 400
func TestCartHandler_AddItem_ZeroQty(t *testing.T) {
	customerID := uuid.New()
	productID := uuid.New()
	fakeCart := newFakeCart()
	h := NewCartHandler(fakeCart)
	r := buildCartRouter(h)

	body := map[string]interface{}{
		"product_id": productID,
		"qty":        0,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/cart/items", bytes.NewReader(bodyBytes))
	req = withCID(req, customerID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var errResp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}

	if errResp["error"] != "qty must be positive" {
		t.Errorf("expected error 'qty must be positive', got '%s'", errResp["error"])
	}
}

// TestCartHandler_RemoveItem tests removing an item from the cart
func TestCartHandler_RemoveItem(t *testing.T) {
	customerID := uuid.New()
	productID := uuid.New()
	fakeCart := newFakeCart()
	h := NewCartHandler(fakeCart)
	r := buildCartRouter(h)

	// First add an item
	if f := fakeCart.items[customerID]; f == nil {
		fakeCart.items[customerID] = make(map[uuid.UUID]int)
	}
	fakeCart.items[customerID][productID] = 5

	// Now delete it
	req := httptest.NewRequest(http.MethodDelete, "/cart/items/"+productID.String(), nil)
	req = withCID(req, customerID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var v srv.CartView
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(v.Items) != 0 {
		t.Errorf("expected empty cart after deletion, got %d items", len(v.Items))
	}
}
