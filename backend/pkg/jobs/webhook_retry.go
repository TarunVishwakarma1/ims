package jobs

import (
	"context"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/internal/service"
	"go.uber.org/zap"
)

// StartWebhookRetry polls webhook_events for failed events whose next_retry_at
// has passed and re-runs them through the payment service. Events that exceed
// the max attempts are sent to the DLQ by the service itself.
func StartWebhookRetry(
	ctx context.Context,
	webhookRepo repository.WebhookRepository,
	paymentSvc service.PaymentService,
	interval time.Duration,
) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				zap.L().Info("webhook retry job stopped")
				return
			case <-ticker.C:
				runOnePass(ctx, webhookRepo, paymentSvc)
			}
		}
	}()
	zap.L().Info("webhook retry job started", zap.Duration("interval", interval))
}

func runOnePass(ctx context.Context, repo repository.WebhookRepository, svc service.PaymentService) {
	// Pull DLQ depth for the metric. We don't have a dedicated "ready to retry"
	// counter — that work would live in a future repo method.
	// Retry pass is currently a no-op placeholder; once we have a `ListDueRetries`
	// method on the webhook repo we can iterate and invoke ProcessWebhook again.
	_ = repo
	_ = svc
}
