package shop

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	srv "github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
)

// fakeCheckoutPaymentOptions is a test double for srv.CheckoutService that
// allows controlling what PaymentOptions returns.
type fakeCheckoutPaymentOptions struct {
	opts []srv.PaymentOption
	err  error
}

func (f *fakeCheckoutPaymentOptions) Summary(_ context.Context, _, _ uuid.UUID, _ string) (*srv.CheckoutSummary, error) {
	return nil, nil
}

func (f *fakeCheckoutPaymentOptions) Place(_ context.Context, _ srv.PlaceOrderInput) (*srv.PlaceOrderResult, error) {
	return nil, nil
}

func (f *fakeCheckoutPaymentOptions) PaymentOptions(_ context.Context, _ uuid.UUID) ([]srv.PaymentOption, error) {
	return f.opts, f.err
}

func withCIDPaymentOptions(req *http.Request, cid uuid.UUID) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), middleware.ContextKeyCustomerID, cid))
}

// paymentOptionsResponse is the expected JSON shape of the handler response.
type paymentOptionsResponse struct {
	Methods []srv.PaymentOption `json:"methods"`
}

// TestPaymentOptions_EmptyCart_BothEnabled: empty cart → both methods enabled, no reason.
func TestPaymentOptions_EmptyCart_BothEnabled(t *testing.T) {
	fake := &fakeCheckoutPaymentOptions{
		opts: []srv.PaymentOption{
			{ID: "razorpay", Enabled: true},
			{ID: "cod", Enabled: true, MinPaise: 10000, MaxPaise: 500000},
		},
	}
	h := NewCheckoutHandler(fake, nil)

	req := httptest.NewRequest(http.MethodGet, "/checkout/payment-options", nil)
	req = withCIDPaymentOptions(req, uuid.New())
	rec := httptest.NewRecorder()
	h.PaymentOptions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var got paymentOptionsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(got.Methods) != 2 {
		t.Fatalf("expected 2 methods, got %d", len(got.Methods))
	}
	codOpt := findMethod(got.Methods, "cod")
	if codOpt == nil {
		t.Fatal("expected 'cod' method in response")
	}
	if !codOpt.Enabled {
		t.Errorf("expected cod enabled for empty cart, got disabled (reason=%q)", codOpt.Reason)
	}
	if codOpt.Reason != "" {
		t.Errorf("expected empty reason for enabled cod, got %q", codOpt.Reason)
	}
}

// TestPaymentOptions_CartUnderMin_CODDisabled_min_value_below: cart total below
// COD min → cod disabled with reason "min_value_below".
func TestPaymentOptions_CartUnderMin_CODDisabled_min_value_below(t *testing.T) {
	fake := &fakeCheckoutPaymentOptions{
		opts: []srv.PaymentOption{
			{ID: "razorpay", Enabled: true},
			{ID: "cod", Enabled: false, MinPaise: 10000, MaxPaise: 500000, Reason: "min_value_below"},
		},
	}
	h := NewCheckoutHandler(fake, nil)

	req := httptest.NewRequest(http.MethodGet, "/checkout/payment-options", nil)
	req = withCIDPaymentOptions(req, uuid.New())
	rec := httptest.NewRecorder()
	h.PaymentOptions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var got paymentOptionsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	codOpt := findMethod(got.Methods, "cod")
	if codOpt == nil {
		t.Fatal("expected 'cod' method in response")
	}
	if codOpt.Enabled {
		t.Error("expected cod disabled when cart is below min")
	}
	if codOpt.Reason != "min_value_below" {
		t.Errorf("expected reason 'min_value_below', got %q", codOpt.Reason)
	}
}

// TestPaymentOptions_CartOverMax_CODDisabled_max_value_exceeded: cart total over
// COD max → cod disabled with reason "max_value_exceeded".
func TestPaymentOptions_CartOverMax_CODDisabled_max_value_exceeded(t *testing.T) {
	fake := &fakeCheckoutPaymentOptions{
		opts: []srv.PaymentOption{
			{ID: "razorpay", Enabled: true},
			{ID: "cod", Enabled: false, MinPaise: 10000, MaxPaise: 500000, Reason: "max_value_exceeded"},
		},
	}
	h := NewCheckoutHandler(fake, nil)

	req := httptest.NewRequest(http.MethodGet, "/checkout/payment-options", nil)
	req = withCIDPaymentOptions(req, uuid.New())
	rec := httptest.NewRecorder()
	h.PaymentOptions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var got paymentOptionsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	codOpt := findMethod(got.Methods, "cod")
	if codOpt == nil {
		t.Fatal("expected 'cod' method in response")
	}
	if codOpt.Enabled {
		t.Error("expected cod disabled when cart exceeds max")
	}
	if codOpt.Reason != "max_value_exceeded" {
		t.Errorf("expected reason 'max_value_exceeded', got %q", codOpt.Reason)
	}
}

// TestPaymentOptions_CartInRange_BothEnabled: cart total within COD bounds →
// both methods enabled, no reason.
func TestPaymentOptions_CartInRange_BothEnabled(t *testing.T) {
	fake := &fakeCheckoutPaymentOptions{
		opts: []srv.PaymentOption{
			{ID: "razorpay", Enabled: true},
			{ID: "cod", Enabled: true, MinPaise: 10000, MaxPaise: 500000},
		},
	}
	h := NewCheckoutHandler(fake, nil)

	req := httptest.NewRequest(http.MethodGet, "/checkout/payment-options", nil)
	req = withCIDPaymentOptions(req, uuid.New())
	rec := httptest.NewRecorder()
	h.PaymentOptions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var got paymentOptionsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	razorOpt := findMethod(got.Methods, "razorpay")
	if razorOpt == nil || !razorOpt.Enabled {
		t.Error("expected razorpay enabled")
	}
	codOpt := findMethod(got.Methods, "cod")
	if codOpt == nil {
		t.Fatal("expected 'cod' method in response")
	}
	if !codOpt.Enabled {
		t.Errorf("expected cod enabled for in-range cart, got disabled (reason=%q)", codOpt.Reason)
	}
	if codOpt.Reason != "" {
		t.Errorf("expected empty reason for enabled cod, got %q", codOpt.Reason)
	}
}

// findMethod is a helper to find a PaymentOption by ID.
func findMethod(opts []srv.PaymentOption, id string) *srv.PaymentOption {
	for i := range opts {
		if opts[i].ID == id {
			return &opts[i]
		}
	}
	return nil
}
