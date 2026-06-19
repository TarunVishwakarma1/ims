package shop_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/internal/testdb"
)

func randPhone() string {
	return fmt.Sprintf("+91600%07d", rand.Intn(10_000_000))
}

func TestCustomerSvc_AddListAddress(t *testing.T) {
	pool := testdb.MustOpen(t)
	custRepo := repository.NewCustomerRepository(pool)
	addrRepo := repository.NewCustomerAddressRepository(pool)
	svc := shop.NewCustomerService(custRepo, addrRepo)
	ctx := context.Background()

	phone := randPhone()
	c, err := custRepo.UpsertByPhone(ctx, phone)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM customers WHERE id = $1`, c.ID)
	})

	_, err = svc.AddAddress(ctx, c.ID, &domain.CustomerAddress{
		Line1:      "L1",
		City:       "Mumbai",
		State:      "MH",
		PostalCode: "400001",
		IsDefault:  true,
	})
	if err != nil {
		t.Fatal(err)
	}

	addrs, err := svc.ListAddresses(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(addrs) != 1 || !addrs[0].IsDefault {
		t.Fatalf("unexpected addresses: %+v", addrs)
	}
}

func TestCustomerSvc_UpdateAddress_Forbidden(t *testing.T) {
	pool := testdb.MustOpen(t)
	custRepo := repository.NewCustomerRepository(pool)
	addrRepo := repository.NewCustomerAddressRepository(pool)
	svc := shop.NewCustomerService(custRepo, addrRepo)
	ctx := context.Background()

	// Create customer A
	phoneA := randPhone()
	custA, err := custRepo.UpsertByPhone(ctx, phoneA)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM customers WHERE id = $1`, custA.ID)
	})

	// Create customer B
	phoneB := randPhone()
	custB, err := custRepo.UpsertByPhone(ctx, phoneB)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM customers WHERE id = $1`, custB.ID)
	})

	// Add address for customer A
	addrID, err := svc.AddAddress(ctx, custA.ID, &domain.CustomerAddress{
		Line1:      "Street A",
		City:       "Delhi",
		State:      "DL",
		PostalCode: "110001",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Customer B tries to update customer A's address → expect "forbidden"
	err = svc.UpdateAddress(ctx, custB.ID, &domain.CustomerAddress{
		ID:         addrID,
		Line1:      "Hacked",
		City:       "Delhi",
		State:      "DL",
		PostalCode: "110001",
	})
	if err == nil || err.Error() != "forbidden" {
		t.Fatalf("expected forbidden, got: %v", err)
	}
}

func TestCustomerSvc_Update(t *testing.T) {
	pool := testdb.MustOpen(t)
	custRepo := repository.NewCustomerRepository(pool)
	addrRepo := repository.NewCustomerAddressRepository(pool)
	svc := shop.NewCustomerService(custRepo, addrRepo)
	ctx := context.Background()

	phone := randPhone()
	c, err := custRepo.UpsertByPhone(ctx, phone)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM customers WHERE id = $1`, c.ID)
	})

	wantName := "Tarun Test"
	wantEmail := "tarun.test@example.com"

	if err = svc.Update(ctx, c.ID, wantName, wantEmail); err != nil {
		t.Fatal(err)
	}

	got, err := svc.Get(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != wantName {
		t.Errorf("name: got %q, want %q", got.Name, wantName)
	}
	if got.Email == nil || *got.Email != wantEmail {
		t.Errorf("email: got %v, want %q", got.Email, wantEmail)
	}
}
