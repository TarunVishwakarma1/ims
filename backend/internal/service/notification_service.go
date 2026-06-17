package service

import (
	"context"
	"errors"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/google/uuid"
)

// NotificationService wraps the notifications outbox for the admin UI.
// Read-mostly; only Resend mutates by flipping a DLQ row back to pending
// so the worker picks it up on its next tick.
type NotificationService interface {
	ListByStatus(ctx context.Context, status domain.NotificationStatus) ([]*domain.Notification, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error)
	// Resend re-queues a failed / DLQ notification for delivery. Resets
	// next_attempt to now so the worker tries immediately on its next
	// ticker firing. Attempts counter is preserved as a paper trail.
	Resend(ctx context.Context, id uuid.UUID) error
}

type notificationService struct {
	repo repository.NotificationRepository
}

func NewNotificationService(r repository.NotificationRepository) NotificationService {
	return &notificationService{repo: r}
}

func (s *notificationService) ListByStatus(ctx context.Context, status domain.NotificationStatus) ([]*domain.Notification, error) {
	return s.repo.ListByStatus(ctx, status, 200)
}

func (s *notificationService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error) {
	// Reuse ListByStatus + a filter is silly; cheaper: query DLQ + pending
	// then find. But we already have raw access via the repo's query, so
	// add a direct lookup later. For now reuse the patterns.
	for _, st := range []domain.NotificationStatus{
		domain.NotificationStatusPending,
		domain.NotificationStatusSent,
		domain.NotificationStatusFailed,
		domain.NotificationStatusDLQ,
	} {
		rows, err := s.repo.ListByStatus(ctx, st, 500)
		if err != nil {
			return nil, err
		}
		for _, n := range rows {
			if n.ID == id {
				return n, nil
			}
		}
	}
	return nil, domain.ErrNotFound
}

func (s *notificationService) Resend(ctx context.Context, id uuid.UUID) error {
	n, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if n.Status == domain.NotificationStatusSent {
		return errors.New("notification already sent")
	}
	// Use MarkFailed with status="pending" and an immediate next_attempt.
	// Caveat: MarkFailed bumps attempts; that's actually what we want here —
	// resends still count toward the retry ledger.
	return s.repo.MarkFailed(ctx, id, "manual resend", time.Now().UTC(), false)
}
