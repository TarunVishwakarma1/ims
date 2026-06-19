package shop

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	srv "github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
)

// fakeCheckout is a test double for srv.CheckoutService.
type fakeCheckout struct {
	summaryResult *srv.CheckoutSummary
	summaryErr    error
	placeResult   *srv.PlaceOrderResult
	placeErr      error
}

func (f *fakeCheckout) Summary(_ context.Context, _, _ uuid.UUID) (*srv.CheckoutSummary, error) {
	return f.summaryResult, f.summaryErr
}

func (f *fakeCheckout) Place(_ context.Context, _ srv.PlaceOrderInput) (*srv.PlaceOrderResult, error) {
	return f.placeResult, f.placeErr
}

// withCIDCheckout injects a customer ID into the request context.
// (mirrors withCID from cart_handler_test.go; same package so alias it locally)
func withCIDCheckout(req *http.Request, cid uuid.UUID) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), middleware.ContextKeyCustomerID, cid))
}

// ---- tests ----

// TestCheckoutHandler_Summary: fakeCheckout returns a summary → 200 + body has expected total.
func TestCheckoutHandler_Summary(t *testing.T) {
	addrID := uuid.New()
	cid := uuid.New()
	expected := &srv.CheckoutSummary{
		Items:             []srv.CartItemView{{ProductID: uuid.New(), Qty: 2, UnitPricePaise: 50000}},
		SubtotalPaise:     100000,
		GSTPaise:          18000,
		ShippingPaise:     0,
		TotalPayablePaise: 118000,
	}

	fake := &fakeCheckout{summaryResult: expected}
	h := NewCheckoutHandler(fake)

	req := httptest.NewRequest(http.MethodGet, "/checkout/summary?address_id="+addrID.String(), nil)
	req = withCIDCheckout(req, cid)
	rec := httptest.NewRecorder()
	h.Summary(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var got srv.CheckoutSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if got.TotalPayablePaise != expected.TotalPayablePaise {
		t.Errorf("expected total %d, got %d", expected.TotalPayablePaise, got.TotalPayablePaise)
	}
}

// TestCheckoutHandler_Summary_MissingAddress: no address_id query param → 400 + "address_required".
func TestCheckoutHandler_Summary_MissingAddress(t *testing.T) {
	fake := &fakeCheckout{}
	h := NewCheckoutHandler(fake)

	req := httptest.NewRequest(http.MethodGet, "/checkout/summary", nil)
	req = withCIDCheckout(req, uuid.New())
	rec := httptest.NewRecorder()
	h.Summary(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var errResp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if errResp["error"] != "address_required" {
		t.Errorf("expected error 'address_required', got '%s'", errResp["error"])
	}
}

// TestCheckoutHandler_Place_COD: fake returns a PlaceOrderResult with InvoiceNumber set → 200 + body matches.
func TestCheckoutHandler_Place_COD(t *testing.T) {
	cid := uuid.New()
	addrID := uuid.New()
	orderID := uuid.New()
	expected := &srv.PlaceOrderResult{
		OrderID:       orderID,
		PayablePaise:  118000,
		InvoiceNumber: "INV-2025-001",
	}

	fake := &fakeCheckout{placeResult: expected}
	h := NewCheckoutHandler(fake)

	body := map[string]interface{}{
		"address_id":     addrID,
		"payment_method": "cod",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/checkout/place", bytes.NewReader(bodyBytes))
	req = withCIDCheckout(req, cid)
	rec := httptest.NewRecorder()
	h.Place(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var got srv.PlaceOrderResult
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if got.InvoiceNumber != expected.InvoiceNumber {
		t.Errorf("expected invoice %s, got %s", expected.InvoiceNumber, got.InvoiceNumber)
	}
	if got.OrderID != expected.OrderID {
		t.Errorf("expected order_id %s, got %s", expected.OrderID, got.OrderID)
	}
}

// TestCheckoutHandler_Place_StockUnavailable: fake returns ErrInsufficientStock → 409 + "stock_unavailable".
func TestCheckoutHandler_Place_StockUnavailable(t *testing.T) {
	cid := uuid.New()
	addrID := uuid.New()

	fake := &fakeCheckout{placeErr: srv.ErrInsufficientStock}
	h := NewCheckoutHandler(fake)

	body := map[string]interface{}{
		"address_id":     addrID,
		"payment_method": "cod",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/checkout/place", bytes.NewReader(bodyBytes))
	req = withCIDCheckout(req, cid)
	rec := httptest.NewRecorder()
	h.Place(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var errResp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if errResp["error"] != "stock_unavailable" {
		t.Errorf("expected error 'stock_unavailable', got '%s'", errResp["error"])
	}
}
