package repository_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/internal/testdb"
)

func TestCustomerRepo_UpsertByPhone_CreatesThenReuses(t *testing.T) {
	pool := testdb.MustOpen(t)
	repo := repository.NewCustomerRepository(pool)
	ctx := context.Background()

	phone := fmt.Sprintf("+91888%07d", rand.Intn(10_000_000))
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM customers WHERE phone = $1`, phone)
	})

	c1, err := repo.UpsertByPhone(ctx, phone)
	if err != nil {
		t.Fatal(err)
	}
	if c1.ID.String() == "" {
		t.Fatal("expected id")
	}
	if !c1.IsVerified {
		t.Fatal("expected verified")
	}

	c2, err := repo.UpsertByPhone(ctx, phone)
	if err != nil {
		t.Fatal(err)
	}
	if c2.ID != c1.ID {
		t.Fatalf("expected same id, got %v vs %v", c1.ID, c2.ID)
	}
}

func TestCustomerRepo_FindByPhone_Missing(t *testing.T) {
	pool := testdb.MustOpen(t)
	repo := repository.NewCustomerRepository(pool)

	c, err := repo.FindByPhone(context.Background(), "+910000000000")
	if err != nil {
		t.Fatal(err)
	}
	if c != nil {
		t.Fatal("expected nil for missing phone")
	}
}

func TestCustomerRepo_FindByPhone_Found(t *testing.T) {
	pool := testdb.MustOpen(t)
	repo := repository.NewCustomerRepository(pool)
	ctx := context.Background()

	phone := fmt.Sprintf("+91777%07d", rand.Intn(10_000_000))
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM customers WHERE phone = $1`, phone)
	})

	created, err := repo.UpsertByPhone(ctx, phone)
	if err != nil {
		t.Fatal(err)
	}

	found, err := repo.FindByPhone(ctx, phone)
	if err != nil {
		t.Fatal(err)
	}
	if found == nil {
		t.Fatal("expected customer, got nil")
	}
	if found.ID != created.ID {
		t.Fatalf("id mismatch: %v vs %v", found.ID, created.ID)
	}
}
