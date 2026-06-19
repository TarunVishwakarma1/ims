package shop_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	shophandler "github.com/TarunVishwakarma1/ims/backend/internal/handler/shop"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
)

// fakeCust is an in-memory implementation of srv.CustomerService for testing.
type fakeCust struct {
	customers  map[uuid.UUID]*domain.Customer
	addresses  map[uuid.UUID][]domain.CustomerAddress // keyed by customer_id
	nextAddrID func() uuid.UUID
}

func newFakeCust() *fakeCust {
	return &fakeCust{
		customers:  make(map[uuid.UUID]*domain.Customer),
		addresses:  make(map[uuid.UUID][]domain.CustomerAddress),
		nextAddrID: uuid.New,
	}
}

func (f *fakeCust) Get(ctx context.Context, id uuid.UUID) (*domain.Customer, error) {
	if c, ok := f.customers[id]; ok {
		return c, nil
	}
	return nil, domain.ErrNotFound
}

func (f *fakeCust) Update(ctx context.Context, id uuid.UUID, name, email string) error {
	c, ok := f.customers[id]
	if !ok {
		return domain.ErrNotFound
	}
	c.Name = name
	if email != "" {
		c.Email = &email
	}
	return nil
}

func (f *fakeCust) AddAddress(ctx context.Context, customerID uuid.UUID, a *domain.CustomerAddress) (uuid.UUID, error) {
	id := f.nextAddrID()
	a.ID = id
	a.CustomerID = customerID
	f.addresses[customerID] = append(f.addresses[customerID], *a)
	return id, nil
}

func (f *fakeCust) ListAddresses(ctx context.Context, customerID uuid.UUID) ([]domain.CustomerAddress, error) {
	return f.addresses[customerID], nil
}

func (f *fakeCust) UpdateAddress(ctx context.Context, customerID uuid.UUID, a *domain.CustomerAddress) error {
	for i, existing := range f.addresses[customerID] {
		if existing.ID == a.ID {
			a.CustomerID = customerID
			f.addresses[customerID][i] = *a
			return nil
		}
	}
	return errors.New("forbidden")
}

func (f *fakeCust) DeleteAddress(ctx context.Context, customerID, addrID uuid.UUID) error {
	addrs := f.addresses[customerID]
	for i, a := range addrs {
		if a.ID == addrID {
			f.addresses[customerID] = append(addrs[:i], addrs[i+1:]...)
			return nil
		}
	}
	return nil
}

func (f *fakeCust) SetDefaultAddress(ctx context.Context, customerID, addrID uuid.UUID) error {
	addrs := f.addresses[customerID]
	for i := range addrs {
		addrs[i].IsDefault = (addrs[i].ID == addrID)
	}
	f.addresses[customerID] = addrs
	return nil
}

// buildRouter wires all customer handler routes onto a chi.Mux.
func buildRouter(h *shophandler.CustomerHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/me", h.GetMe)
	r.Patch("/me", h.UpdateMe)
	r.Get("/addresses", h.ListAddresses)
	r.Post("/addresses", h.AddAddress)
	r.Patch("/addresses/{id}", h.UpdateAddress)
	r.Delete("/addresses/{id}", h.DeleteAddress)
	r.Post("/addresses/{id}/default", h.SetDefaultAddress)
	return r
}

// withCID injects a customer UUID into the request context.
func withCID(req *http.Request, cid uuid.UUID) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), middleware.ContextKeyCustomerID, cid))
}

// --- Tests ---

func TestCustomerHandler_GetMe(t *testing.T) {
	fc := newFakeCust()
	cid := uuid.New()
	fc.customers[cid] = &domain.Customer{ID: cid, Name: "Alice"}

	h := shophandler.NewCustomerHandler(fc)
	r := buildRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req = withCID(req, cid)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var got domain.Customer
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if got.ID != cid {
		t.Fatalf("expected id %s, got %s", cid, got.ID)
	}
	if got.Name != "Alice" {
		t.Fatalf("expected name Alice, got %s", got.Name)
	}
}

func TestCustomerHandler_GetMe_NotFound(t *testing.T) {
	fc := newFakeCust() // empty store
	cid := uuid.New()

	h := shophandler.NewCustomerHandler(fc)
	r := buildRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req = withCID(req, cid)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Error string `json:"error"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Error != "not_found" {
		t.Fatalf("expected error 'not_found', got '%s'", resp.Error)
	}
}

func TestCustomerHandler_AddAddress(t *testing.T) {
	fc := newFakeCust()
	cid := uuid.New()

	h := shophandler.NewCustomerHandler(fc)
	r := buildRouter(h)

	body := bytes.NewReader([]byte(`{"line1":"123 Main St","city":"Mumbai","state":"MH","postal_code":"400001"}`))
	req := httptest.NewRequest(http.MethodPost, "/addresses", body)
	req = withCID(req, cid)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		ID uuid.UUID `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.ID == uuid.Nil {
		t.Fatal("expected a non-nil UUID in response")
	}

	// Verify address was stored
	addrs := fc.addresses[cid]
	if len(addrs) != 1 {
		t.Fatalf("expected 1 address in store, got %d", len(addrs))
	}
}

func TestCustomerHandler_AddAddress_MissingFields(t *testing.T) {
	fc := newFakeCust()
	cid := uuid.New()

	h := shophandler.NewCustomerHandler(fc)
	r := buildRouter(h)

	// line1 is empty
	body := bytes.NewReader([]byte(`{"city":"Mumbai","state":"MH","postal_code":"400001"}`))
	req := httptest.NewRequest(http.MethodPost, "/addresses", body)
	req = withCID(req, cid)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Error string `json:"error"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Error != "missing_fields" {
		t.Fatalf("expected error 'missing_fields', got '%s'", resp.Error)
	}
}

func TestCustomerHandler_UpdateAddress_Forbidden(t *testing.T) {
	fc := newFakeCust()
	cid := uuid.New()

	h := shophandler.NewCustomerHandler(fc)
	r := buildRouter(h)

	// Use a random address ID that doesn't exist — fakeCust.UpdateAddress returns "forbidden"
	addrID := uuid.New()
	body := bytes.NewReader([]byte(`{"line1":"X","city":"Y","state":"Z","postal_code":"000"}`))
	req := httptest.NewRequest(http.MethodPatch, "/addresses/"+addrID.String(), body)
	req = withCID(req, cid)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Error string `json:"error"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Error != "forbidden" {
		t.Fatalf("expected error 'forbidden', got '%s'", resp.Error)
	}
}

func TestCustomerHandler_DeleteAddress(t *testing.T) {
	fc := newFakeCust()
	cid := uuid.New()

	h := shophandler.NewCustomerHandler(fc)
	r := buildRouter(h)

	// First add an address via POST
	addBody := bytes.NewReader([]byte(`{"line1":"456 Park Ave","city":"Delhi","state":"DL","postal_code":"110001"}`))
	addReq := httptest.NewRequest(http.MethodPost, "/addresses", addBody)
	addReq = withCID(addReq, cid)
	addRec := httptest.NewRecorder()
	r.ServeHTTP(addRec, addReq)
	if addRec.Code != http.StatusOK {
		t.Fatalf("add address: expected 200, got %d; body: %s", addRec.Code, addRec.Body.String())
	}

	var addResp struct {
		ID uuid.UUID `json:"id"`
	}
	json.Unmarshal(addRec.Body.Bytes(), &addResp)

	// Delete the address
	delReq := httptest.NewRequest(http.MethodDelete, "/addresses/"+addResp.ID.String(), nil)
	delReq = withCID(delReq, cid)
	delRec := httptest.NewRecorder()
	r.ServeHTTP(delRec, delReq)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d; body: %s", delRec.Code, delRec.Body.String())
	}

	// Verify store is empty
	if len(fc.addresses[cid]) != 0 {
		t.Fatalf("expected 0 addresses after delete, got %d", len(fc.addresses[cid]))
	}
}
