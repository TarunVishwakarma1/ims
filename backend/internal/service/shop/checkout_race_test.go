package shop_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/internal/testdb"
	"github.com/google/uuid"
)

// TestCheckoutSvc_Race_LastUnitExactlyOneWinner seeds a product with stock=1,
// then has N=50 concurrent customers each attempt to check out qty=1.
// Exactly one goroutine must win; the rest must get ErrInsufficientStock.
// Final inventory.quantity must be 0.
func TestCheckoutSvc_Race_LastUnitExactlyOneWinner(t *testing.T) {
	pool := testdb.MustOpen(t)
	ctx := context.Background()

	prodID, orgID := testdb.SeedProductWithStock(t, pool, "RaceUnit", 1000, 1)

	cartRepo := repository.NewCartRepository(pool)
	custRepo := repository.NewCustomerRepository(pool)
	addrRepo := repository.NewCustomerAddressRepository(pool)
	orderRepo := repository.NewOrderRepository(pool)
	cartSvc := shop.NewCartService(cartRepo, pool, orgID)
	checkSvc := shop.NewCheckoutService(pool, orgID, cartRepo, addrRepo, nil, orderRepo, "", 0, 1<<31)

	const N = 50
	type seed struct {
		customerID uuid.UUID
		addressID  uuid.UUID
	}
	seeds := make([]seed, N)

	for i := 0; i < N; i++ {
		// Use a counter suffix to guarantee unique phone numbers.
		phone := fmt.Sprintf("+91700%08d", i+1)
		c, err := custRepo.UpsertByPhone(ctx, phone)
		if err != nil {
			t.Fatalf("seed customer %d: %v", i, err)
		}
		ii := i
		t.Cleanup(func() {
			_, _ = pool.Exec(ctx, `DELETE FROM customers WHERE id=$1`, c.ID)
		})

		aID, err := addrRepo.Create(ctx, &domain.CustomerAddress{
			CustomerID: c.ID,
			Label:      "Home",
			Line1:      fmt.Sprintf("Line %d", ii+1),
			City:       "Mumbai",
			State:      "MH",
			PostalCode: "400001",
			IsDefault:  false,
		})
		if err != nil {
			t.Fatalf("seed addr %d: %v", ii, err)
		}
		t.Cleanup(func() {
			_, _ = pool.Exec(ctx, `DELETE FROM customer_addresses WHERE id=$1`, aID)
		})

		if _, err := cartSvc.AddOrSet(ctx, c.ID, prodID, 1); err != nil {
			t.Fatalf("seed cart %d: %v", ii, err)
		}

		seeds[ii] = seed{c.ID, aID}
	}

	var winners atomic.Int32
	var losers atomic.Int32
	var wg sync.WaitGroup
	wg.Add(N)

	for i := 0; i < N; i++ {
		i := i
		go func() {
			defer wg.Done()
			_, err := checkSvc.Place(context.Background(), shop.PlaceOrderInput{
				CustomerID:    seeds[i].customerID,
				AddressID:     seeds[i].addressID,
				PaymentMethod: "cod",
			})
			if err == nil {
				winners.Add(1)
			} else if err == shop.ErrInsufficientStock {
				losers.Add(1)
			} else {
				t.Errorf("goroutine %d unexpected error: %v", i, err)
			}
		}()
	}
	wg.Wait()

	if winners.Load() != 1 {
		t.Fatalf("expected exactly 1 winner, got %d", winners.Load())
	}
	if losers.Load() != N-1 {
		t.Fatalf("expected %d losers, got %d", N-1, losers.Load())
	}

	// Verify final inventory quantity is 0.
	if got := testdb.StockOf(t, pool, prodID); got != 0 {
		t.Fatalf("expected stock=0 after race, got %d", got)
	}
}
