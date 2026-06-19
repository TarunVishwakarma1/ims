package shop_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/internal/testdb"
	"github.com/TarunVishwakarma1/ims/backend/pkg/sms"
)

func randOTPPhone() string {
	return fmt.Sprintf("+9199999%05d", rand.Intn(100_000))
}

func newOTPSvc(t *testing.T) (shop.OTPService, *sms.MockSender) {
	t.Helper()
	pool := testdb.MustOpen(t)
	custRepo := repository.NewCustomerRepository(pool)
	otpRepo := repository.NewOTPRepository(pool)
	mock := &sms.MockSender{}
	svc := shop.NewOTPService(otpRepo, custRepo, mock, "jwt-secret")
	return svc, mock
}

func cleanupOTPPhone(t *testing.T, pool interface{ Exec(ctx context.Context, sql string, args ...any) (interface{ RowsAffected() int64 }, error) }, phone string) {
	t.Helper()
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM otp_sessions WHERE phone = $1`, phone) //nolint:errcheck
		pool.Exec(context.Background(), `DELETE FROM customers WHERE phone = $1`, phone)     //nolint:errcheck
	})
}

func TestOTPSvc_SendAndVerifyHappy(t *testing.T) {
	pool := testdb.MustOpen(t)
	custRepo := repository.NewCustomerRepository(pool)
	otpRepo := repository.NewOTPRepository(pool)
	mock := &sms.MockSender{}
	svc := shop.NewOTPService(otpRepo, custRepo, mock, "jwt-secret")

	phone := randOTPPhone()
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM otp_sessions WHERE phone = $1`, phone)
		pool.Exec(context.Background(), `DELETE FROM customers WHERE phone = $1`, phone)
	})

	id, exp, err := svc.Send(context.Background(), phone, "login")
	if err != nil {
		t.Fatal(err)
	}
	if exp != 300 {
		t.Fatalf("expected 300s expiry, got %d", exp)
	}
	if len(mock.Sent) != 1 {
		t.Fatalf("expected 1 sms, got %d", len(mock.Sent))
	}

	code := mock.Sent[0].Code
	cid, tok, err := svc.Verify(context.Background(), id, code)
	if err != nil {
		t.Fatal(err)
	}
	if cid == [16]byte{} {
		t.Fatal("expected non-nil customer id")
	}
	if tok == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestOTPSvc_VerifyWrongCode(t *testing.T) {
	pool := testdb.MustOpen(t)
	custRepo := repository.NewCustomerRepository(pool)
	otpRepo := repository.NewOTPRepository(pool)
	mock := &sms.MockSender{}
	svc := shop.NewOTPService(otpRepo, custRepo, mock, "jwt-secret")

	phone := randOTPPhone()
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM otp_sessions WHERE phone = $1`, phone)
		pool.Exec(context.Background(), `DELETE FROM customers WHERE phone = $1`, phone)
	})

	id, _, err := svc.Send(context.Background(), phone, "login")
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = svc.Verify(context.Background(), id, "000000")
	if err != shop.ErrInvalidCode {
		t.Fatalf("expected ErrInvalidCode, got %v", err)
	}
}

func TestOTPSvc_HourlyRateLimit(t *testing.T) {
	pool := testdb.MustOpen(t)
	custRepo := repository.NewCustomerRepository(pool)
	otpRepo := repository.NewOTPRepository(pool)
	mock := &sms.MockSender{}
	svc := shop.NewOTPService(otpRepo, custRepo, mock, "jwt-secret")

	phone := randOTPPhone()
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM otp_sessions WHERE phone = $1`, phone)
		pool.Exec(context.Background(), `DELETE FROM customers WHERE phone = $1`, phone)
	})

	for i := 0; i < 5; i++ {
		if _, _, err := svc.Send(context.Background(), phone, "login"); err != nil {
			t.Fatalf("send #%d failed: %v", i+1, err)
		}
	}

	_, _, err := svc.Send(context.Background(), phone, "login")
	if err != shop.ErrRateLimited {
		t.Fatalf("expected ErrRateLimited on 6th send, got %v", err)
	}
}

func TestOTPSvc_VerifyLockoutAfterFiveBadAttempts(t *testing.T) {
	pool := testdb.MustOpen(t)
	custRepo := repository.NewCustomerRepository(pool)
	otpRepo := repository.NewOTPRepository(pool)
	mock := &sms.MockSender{}
	svc := shop.NewOTPService(otpRepo, custRepo, mock, "jwt-secret")

	phone := randOTPPhone()
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM otp_sessions WHERE phone = $1`, phone)
		pool.Exec(context.Background(), `DELETE FROM customers WHERE phone = $1`, phone)
	})

	id, _, err := svc.Send(context.Background(), phone, "login")
	if err != nil {
		t.Fatal(err)
	}

	// Wrong code 5 times — the 5th should return ErrOTPLocked
	var lastErr error
	for i := 0; i < 5; i++ {
		_, _, lastErr = svc.Verify(context.Background(), id, "000000")
	}
	if lastErr != shop.ErrOTPLocked {
		t.Fatalf("expected ErrOTPLocked after 5 bad attempts, got %v", lastErr)
	}

	// Further call must still return ErrOTPLocked
	_, _, err = svc.Verify(context.Background(), id, "000000")
	if err != shop.ErrOTPLocked {
		t.Fatalf("expected ErrOTPLocked on locked session, got %v", err)
	}
}
