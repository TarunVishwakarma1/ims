package jobs

import (
	"context"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/pkg/notify"
	"go.uber.org/zap"
)

// Max retries before a notification gets parked in DLQ. Each attempt
// is spaced via backoff schedule below; total horizon ≈ 1 hour.
const notificationMaxAttempts = 5

// notificationBackoff returns the wait duration before retry N (1-indexed).
// 30s → 2m → 10m → 30m → 1h, then DLQ.
func notificationBackoff(attempt int) time.Duration {
	schedule := []time.Duration{
		30 * time.Second,
		2 * time.Minute,
		10 * time.Minute,
		30 * time.Minute,
		1 * time.Hour,
	}
	if attempt < 1 {
		attempt = 1
	}
	if attempt > len(schedule) {
		attempt = len(schedule)
	}
	return schedule[attempt-1]
}

// StartNotificationWorker drains the notifications table. Picks up pending
// rows whose next_attempt is due, attempts to send via the Emailer, marks
// sent or schedules retry / DLQ based on the failure.
//
// Single-threaded by design — keeps the SMTP server happy and avoids
// per-row locking. If throughput becomes a bottleneck, add SELECT ... FOR
// UPDATE SKIP LOCKED and run multiple workers.
func StartNotificationWorker(ctx context.Context, notRepo repository.NotificationRepository, em notify.Emailer, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				zap.L().Info("notification worker stopped")
				return
			case <-ticker.C:
				runNotificationBatch(ctx, notRepo, em)
			}
		}
	}()
	zap.L().Info("notification worker started", zap.Duration("interval", interval))
}

func runNotificationBatch(ctx context.Context, notRepo repository.NotificationRepository, em notify.Emailer) {
	rows, err := notRepo.ListDue(ctx, 50)
	if err != nil {
		zap.L().Warn("notification worker: ListDue failed", zap.Error(err))
		return
	}
	for _, n := range rows {
		dispatchOne(ctx, notRepo, em, n)
	}
}

func dispatchOne(ctx context.Context, notRepo repository.NotificationRepository, em notify.Emailer, n *domain.Notification) {
	body := n.BodyText
	var html string
	if n.BodyHTML != nil {
		html = *n.BodyHTML
	}
	err := em.Send([]string{n.Recipient}, n.Subject, body, html)
	if err == nil {
		if mErr := notRepo.MarkSent(ctx, n.ID); mErr != nil {
			zap.L().Warn("MarkSent failed", zap.String("id", n.ID.String()), zap.Error(mErr))
		}
		return
	}
	// Failure path — bump attempts, decide retry vs DLQ.
	attempts := n.Attempts + 1
	dlq := attempts >= notificationMaxAttempts
	next := time.Now().Add(notificationBackoff(attempts)).UTC()
	if mErr := notRepo.MarkFailed(ctx, n.ID, err.Error(), next, dlq); mErr != nil {
		zap.L().Warn("MarkFailed failed",
			zap.String("id", n.ID.String()), zap.Error(mErr))
	}
	zap.L().Warn("notification delivery failed",
		zap.String("id", n.ID.String()),
		zap.String("to", n.Recipient),
		zap.Int("attempts", attempts),
		zap.Bool("dlq", dlq),
		zap.Error(err))
}
