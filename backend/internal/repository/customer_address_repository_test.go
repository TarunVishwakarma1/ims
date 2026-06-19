package repository_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/internal/testdb"
	"github.com/google/uuid"
)

// randomPhone generates a unique phone number to avoid run-to-run collisions.
func randomPhone() string {
	return fmt.Sprintf("+91600%07d", rand.Intn(10_000_000))
}

// TestAddressRepo_SetDefault_ClearsOthers is the brief-mandated test.
func TestAddressRepo_SetDefault_ClearsOthers(t *testing.T) {
	pool := testdb.MustOpen(t)
	custRepo := repository.NewCustomerRepository(pool)
	addrRepo := repository.NewCustomerAddressRepository(pool)
	ctx := context.Background()

	phone := randomPhone()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM customers WHERE phone = $1`, phone)
	})
	c, err := custRepo.UpsertByPhone(ctx, phone)
	if err != nil {
		t.Fatal(err)
	}

	a1ID, err := addrRepo.Create(ctx, &domain.CustomerAddress{
		CustomerID: c.ID, Line1: "A1", City: "Mumbai", State: "MH", PostalCode: "400001", IsDefault: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	a2ID, err := addrRepo.Create(ctx, &domain.CustomerAddress{
		CustomerID: c.ID, Line1: "A2", City: "Pune", State: "MH", PostalCode: "411001",
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := addrRepo.SetDefault(ctx, a2ID, c.ID); err != nil {
		t.Fatal(err)
	}

	list, err := addrRepo.ListByCustomer(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	var got1Def, got2Def bool
	for _, a := range list {
		if a.ID == a1ID {
			got1Def = a.IsDefault
		}
		if a.ID == a2ID {
			got2Def = a.IsDefault
		}
	}
	if got1Def {
		t.Fatal("a1 should no longer be default")
	}
	if !got2Def {
		t.Fatal("a2 should be default")
	}
}

// TestAddressRepo_CreateAndList verifies Create returns a retrievable ID and
// ListByCustomer returns the address with correct fields.
func TestAddressRepo_CreateAndList(t *testing.T) {
	pool := testdb.MustOpen(t)
	custRepo := repository.NewCustomerRepository(pool)
	addrRepo := repository.NewCustomerAddressRepository(pool)
	ctx := context.Background()

	phone := randomPhone()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM customers WHERE phone = $1`, phone)
	})
	c, err := custRepo.UpsertByPhone(ctx, phone)
	if err != nil {
		t.Fatal(err)
	}

	want := &domain.CustomerAddress{
		CustomerID: c.ID,
		Label:      "Office",
		Line1:      "123 MG Road",
		Line2:      "Floor 2",
		City:       "Bangalore",
		State:      "KA",
		Country:    "IN",
		PostalCode: "560001",
		IsDefault:  false,
	}
	id, err := addrRepo.Create(ctx, want)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id == uuid.Nil {
		t.Fatal("expected non-nil uuid from Create")
	}

	list, err := addrRepo.ListByCustomer(ctx, c.ID)
	if err != nil {
		t.Fatalf("ListByCustomer: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 address, got %d", len(list))
	}
	got := list[0]
	if got.ID != id {
		t.Fatalf("id mismatch: want %v got %v", id, got.ID)
	}
	if got.Line1 != want.Line1 {
		t.Fatalf("Line1 mismatch: want %q got %q", want.Line1, got.Line1)
	}
	if got.Label != want.Label {
		t.Fatalf("Label mismatch: want %q got %q", want.Label, got.Label)
	}
	if got.City != want.City {
		t.Fatalf("City mismatch: want %q got %q", want.City, got.City)
	}
}

// TestAddressRepo_GetByID_NotFound verifies GetByID returns nil for a missing id.
func TestAddressRepo_GetByID_NotFound(t *testing.T) {
	pool := testdb.MustOpen(t)
	addrRepo := repository.NewCustomerAddressRepository(pool)
	ctx := context.Background()

	got, err := addrRepo.GetByID(ctx, uuid.New())
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for non-existent ID, got %+v", got)
	}
}

// TestAddressRepo_Update verifies Update changes fields, and that passing a wrong
// customerID for the address ID results in no rows affected (no error, no change).
func TestAddressRepo_Update(t *testing.T) {
	pool := testdb.MustOpen(t)
	custRepo := repository.NewCustomerRepository(pool)
	addrRepo := repository.NewCustomerAddressRepository(pool)
	ctx := context.Background()

	phone := randomPhone()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM customers WHERE phone = $1`, phone)
	})
	c, err := custRepo.UpsertByPhone(ctx, phone)
	if err != nil {
		t.Fatal(err)
	}

	id, err := addrRepo.Create(ctx, &domain.CustomerAddress{
		CustomerID: c.ID, Line1: "Old Line1", City: "OldCity", State: "MH", PostalCode: "400001",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Update with correct customerID.
	err = addrRepo.Update(ctx, &domain.CustomerAddress{
		ID:         id,
		CustomerID: c.ID,
		Label:      "Updated",
		Line1:      "New Line1",
		City:       "NewCity",
		State:      "KA",
		PostalCode: "560001",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := addrRepo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("expected address, got nil")
	}
	if got.Line1 != "New Line1" {
		t.Fatalf("Line1 not updated: got %q", got.Line1)
	}
	if got.City != "NewCity" {
		t.Fatalf("City not updated: got %q", got.City)
	}

	// Update with wrong customerID — no error, no rows changed.
	err = addrRepo.Update(ctx, &domain.CustomerAddress{
		ID:         id,
		CustomerID: uuid.New(), // wrong customer
		Line1:      "Should Not Change",
		City:       "ShouldNotChange",
	})
	if err != nil {
		t.Fatalf("Update with wrong customerID: unexpected error: %v", err)
	}

	// Verify address is unchanged.
	got2, err := addrRepo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID after wrong-customer update: %v", err)
	}
	if got2.Line1 != "New Line1" {
		t.Fatalf("Line1 was changed by wrong-customer update: got %q", got2.Line1)
	}
}

// TestAddressRepo_Delete verifies Delete is scoped to customerID.
func TestAddressRepo_Delete(t *testing.T) {
	pool := testdb.MustOpen(t)
	custRepo := repository.NewCustomerRepository(pool)
	addrRepo := repository.NewCustomerAddressRepository(pool)
	ctx := context.Background()

	phone := randomPhone()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM customers WHERE phone = $1`, phone)
	})
	c, err := custRepo.UpsertByPhone(ctx, phone)
	if err != nil {
		t.Fatal(err)
	}

	id, err := addrRepo.Create(ctx, &domain.CustomerAddress{
		CustomerID: c.ID, Line1: "ToDelete", City: "City", State: "ST", PostalCode: "000000",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Delete with wrong customerID — address should still exist.
	if err := addrRepo.Delete(ctx, id, uuid.New()); err != nil {
		t.Fatalf("Delete (wrong customer): unexpected error: %v", err)
	}
	got, err := addrRepo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID after wrong-customer delete: %v", err)
	}
	if got == nil {
		t.Fatal("address was deleted by wrong-customer delete — should not have been")
	}

	// Delete with correct customerID — address should be gone.
	if err := addrRepo.Delete(ctx, id, c.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	got2, err := addrRepo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID after delete: %v", err)
	}
	if got2 != nil {
		t.Fatalf("expected address to be deleted, still exists: %+v", got2)
	}
}
