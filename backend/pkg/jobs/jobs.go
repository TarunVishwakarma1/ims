package jobs

import (
	"context"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"go.uber.org/zap"
)

// StartReservationExpiry runs the reservation cleanup periodically.
// Releases reservations whose expires_at has passed and adds quantity back to inventory.
func StartReservationExpiry(ctx context.Context, marketRepo repository.MarketplaceRepository, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// Run once immediately on startup
		runExpiry(ctx, marketRepo)

		for {
			select {
			case <-ctx.Done():
				zap.L().Info("reservation expiry job stopped")
				return
			case <-ticker.C:
				runExpiry(ctx, marketRepo)
			}
		}
	}()
	zap.L().Info("reservation expiry job started", zap.Duration("interval", interval))
}

func runExpiry(ctx context.Context, marketRepo repository.MarketplaceRepository) {
	if err := marketRepo.ExpireStaleReservations(ctx); err != nil {
		zap.L().Error("expire stale reservations failed", zap.Error(err))
	}
}

// StartCartCleanup deletes carts whose expires_at has passed.
// Customers/orgs get a fresh cart on next request.
func StartCartCleanup(ctx context.Context, marketRepo repository.MarketplaceRepository, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				zap.L().Info("cart cleanup job stopped")
				return
			case <-ticker.C:
				// Cart cleanup hook reserved for future implementation.
				// CASCADE DELETE on cart_items handles items if we drop expired carts.
			}
		}
	}()
}

// StartRefreshTokenCleanup deletes refresh tokens 7+ days past their expiry.
func StartRefreshTokenCleanup(ctx context.Context, authRepo repository.AuthRepository, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				zap.L().Info("refresh token cleanup stopped")
				return
			case <-ticker.C:
				if err := authRepo.DeleteExpiredRefreshTokens(ctx); err != nil {
					zap.L().Error("refresh token cleanup failed", zap.Error(err))
				}
			}
		}
	}()
	zap.L().Info("refresh token cleanup job started", zap.Duration("interval", interval))
}
