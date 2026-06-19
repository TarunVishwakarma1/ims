package shop_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/google/uuid"

	shophandler "github.com/TarunVishwakarma1/ims/backend/internal/handler/shop"
	"github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
)

type stubOTP struct {
	sent          map[string]string
	sendErr       error
	verifyErr     error
	verifyOverride bool
}

var phoneRe = regexp.MustCompile(`^\+\d{10,15}$`)

func (s *stubOTP) Send(ctx context.Context, phone, purpose string) (uuid.UUID, int, error) {
	if !phoneRe.MatchString(phone) {
		return uuid.Nil, 0, shop.ErrInvalidPhone
	}
	if s.sendErr != nil {
		return uuid.Nil, 0, s.sendErr
	}
	id := uuid.New()
	s.sent[id.String()] = "111111"
	return id, 300, nil
}

func (s *stubOTP) Verify(ctx context.Context, id uuid.UUID, code string) (uuid.UUID, string, error) {
	if s.verifyErr != nil {
		return uuid.Nil, "", s.verifyErr
	}
	if s.sent[id.String()] != code {
		return uuid.Nil, "", shop.ErrInvalidCode
	}
	return uuid.New(), "tok_test_token", nil
}

func TestAuthHandler_SendVerify(t *testing.T) {
	stub := &stubOTP{sent: map[string]string{}}
	h := shophandler.NewAuthHandler(stub)

	// Send OTP
	rec := httptest.NewRecorder()
	body := bytes.NewReader([]byte(`{"phone":"+919999900901"}`))
	req := httptest.NewRequest("POST", "/api/shop/auth/otp/send", body)
	h.Send(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("send: expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var sendResp struct {
		OTPID     string `json:"otp_id"`
		ExpiresIn int    `json:"expires_in"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &sendResp); err != nil {
		t.Fatalf("send: failed to parse response: %v", err)
	}
	if sendResp.OTPID == "" {
		t.Fatal("send: otp_id is empty")
	}

	// Verify OTP
	rec = httptest.NewRecorder()
	body = bytes.NewReader([]byte(`{"otp_id":"` + sendResp.OTPID + `","code":"111111"}`))
	req = httptest.NewRequest("POST", "/api/shop/auth/otp/verify", body)
	h.Verify(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("verify: expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var verifyResp struct {
		Token    string `json:"token"`
		Customer struct {
			ID string `json:"id"`
		} `json:"customer"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &verifyResp); err != nil {
		t.Fatalf("verify: failed to parse response: %v", err)
	}
	if verifyResp.Token == "" {
		t.Fatal("verify: token is empty")
	}
	if verifyResp.Customer.ID == "" {
		t.Fatal("verify: customer.id is empty")
	}
}

func TestAuthHandler_VerifyInvalidCode(t *testing.T) {
	stub := &stubOTP{sent: map[string]string{}}
	h := shophandler.NewAuthHandler(stub)

	// Send OTP
	rec := httptest.NewRecorder()
	body := bytes.NewReader([]byte(`{"phone":"+919999900901"}`))
	req := httptest.NewRequest("POST", "/api/shop/auth/otp/send", body)
	h.Send(rec, req)

	var sendResp struct {
		OTPID string `json:"otp_id"`
	}
	json.Unmarshal(rec.Body.Bytes(), &sendResp)

	// Verify with wrong code
	rec = httptest.NewRecorder()
	body = bytes.NewReader([]byte(`{"otp_id":"` + sendResp.OTPID + `","code":"000000"}`))
	req = httptest.NewRequest("POST", "/api/shop/auth/otp/verify", body)
	h.Verify(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("verify: expected 400, got %d", rec.Code)
	}

	var errResp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("verify: failed to parse error response: %v", err)
	}
	if errResp.Error != "invalid_code" {
		t.Fatalf("verify: expected 'invalid_code', got '%s'", errResp.Error)
	}
}

func TestAuthHandler_SendInvalidPhone(t *testing.T) {
	stub := &stubOTP{sent: map[string]string{}}
	h := shophandler.NewAuthHandler(stub)

	rec := httptest.NewRecorder()
	body := bytes.NewReader([]byte(`{"phone":"9999900901"}`))
	req := httptest.NewRequest("POST", "/api/shop/auth/otp/send", body)
	h.Send(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("send: expected 400, got %d", rec.Code)
	}

	var errResp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("send: failed to parse error response: %v", err)
	}
	if errResp.Error != "invalid_phone" {
		t.Fatalf("send: expected 'invalid_phone', got '%s'", errResp.Error)
	}
}

func TestAuthHandler_SendRateLimit(t *testing.T) {
	stub := &stubOTP{
		sent:    map[string]string{},
		sendErr: shop.ErrRateLimited,
	}
	h := shophandler.NewAuthHandler(stub)

	rec := httptest.NewRecorder()
	body := bytes.NewReader([]byte(`{"phone":"+919999900901"}`))
	req := httptest.NewRequest("POST", "/api/shop/auth/otp/send", body)
	h.Send(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("send: expected 429, got %d", rec.Code)
	}

	var errResp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("send: failed to parse error response: %v", err)
	}
	if errResp.Error != "rate_limit" {
		t.Fatalf("send: expected 'rate_limit', got '%s'", errResp.Error)
	}
}
