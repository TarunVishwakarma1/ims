package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/internal/testdb"
	"github.com/google/uuid"
)

func TestOTPRepo_CreateAndIncrement(t *testing.T) {
	pool := testdb.MustOpen(t)
	repo := repository.NewOTPRepository(pool)
	ctx := context.Background()

	phone := "+919999900201"
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM otp_sessions WHERE phone = $1`, phone)
	})

	id, err := repo.Create(ctx, &domain.OTPSession{
		Phone:     phone,
		CodeHash:  "h",
		Purpose:   domain.OTPPurposeLogin,
		SentCount: 1,
		ExpiresAt: time.Now().UTC().Add(domain.OTPTTL),
	})
	if err != nil {
		t.Fatal(err)
	}
	if id == uuid.Nil {
		t.Fatal("expected non-nil UUID")
	}

	n1, err := repo.IncrementAttempts(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	n2, err := repo.IncrementAttempts(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if n1 != 1 || n2 != 2 {
		t.Fatalf("expected attempts 1 then 2, got %d %d", n1, n2)
	}

	if err := repo.MarkConsumed(ctx, id); err != nil {
		t.Fatal(err)
	}
	// idempotent: second call must not error
	if err := repo.MarkConsumed(ctx, id); err != nil {
		t.Fatalf("MarkConsumed not idempotent: %v", err)
	}

	s, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if s == nil {
		t.Fatal("expected session, got nil")
	}
	if s.ConsumedAt == nil {
		t.Fatal("expected consumed_at to be set")
	}
}

func TestOTPRepo_CountSends(t *testing.T) {
	pool := testdb.MustOpen(t)
	repo := repository.NewOTPRepository(pool)
	ctx := context.Background()

	phone := "+919999900202"
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM otp_sessions WHERE phone = $1`, phone)
	})

	for i := 0; i < 3; i++ {
		_, err := repo.Create(ctx, &domain.OTPSession{
			Phone:     phone,
			CodeHash:  "h",
			Purpose:   domain.OTPPurposeLogin,
			SentCount: 1,
			ExpiresAt: time.Now().UTC().Add(time.Minute),
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	n, err := repo.CountSends(ctx, phone, time.Now().UTC().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("expected 3, got %d", n)
	}
}

func TestOTPRepo_GetByID_NotFound(t *testing.T) {
	pool := testdb.MustOpen(t)
	repo := repository.NewOTPRepository(pool)
	ctx := context.Background()

	s, err := repo.GetByID(ctx, uuid.New())
	if err != nil {
		t.Fatalf("expected nil error on miss, got: %v", err)
	}
	if s != nil {
		t.Fatalf("expected nil session on miss, got: %+v", s)
	}
}
