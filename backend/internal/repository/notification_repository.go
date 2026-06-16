package repository

import (
	"context"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/google/uuid"
)

type NotificationRepository interface {
	// Enqueue persists a new pending notification ready for the worker.
	Enqueue(ctx context.Context, n *domain.Notification) error

	// ListDue returns up to `limit` pending rows whose next_attempt has
	// passed. Sorted oldest-first so we drain the backlog before new work.
	ListDue(ctx context.Context, limit int) ([]*domain.Notification, error)

	// MarkSent flips a row to status=sent and stamps sent_at.
	MarkSent(ctx context.Context, id uuid.UUID) error

	// MarkFailed bumps attempts, stores the error, and either schedules the
	// next retry or moves the row to DLQ when attempts >= maxAttempts.
	MarkFailed(ctx context.Context, id uuid.UUID, errMsg string, nextAttempt time.Time, dlq bool) error

	// ListByStatus is used by the admin UI (pending queue, failed DLQ).
	ListByStatus(ctx context.Context, status domain.NotificationStatus, limit int) ([]*domain.Notification, error)
}

type notificationRepository struct {
	db DBTX
}

func NewNotificationRepository(db DBTX) NotificationRepository {
	return &notificationRepository{db: db}
}

func (r *notificationRepository) Enqueue(ctx context.Context, n *domain.Notification) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO notifications (id, channel, recipient, subject, body_text, body_html, status, attempts, next_attempt, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, n.ID, n.Channel, n.Recipient, n.Subject, n.BodyText, n.BodyHTML, n.Status, n.Attempts, n.NextAttempt, n.CreatedAt)
	return err
}

const notificationCols = `id, channel, recipient, subject, body_text, body_html,
		status, attempts, last_error, next_attempt, created_at, sent_at`

func scanNotification(scanner interface {
	Scan(dest ...any) error
}) (*domain.Notification, error) {
	n := &domain.Notification{}
	err := scanner.Scan(
		&n.ID, &n.Channel, &n.Recipient, &n.Subject, &n.BodyText, &n.BodyHTML,
		&n.Status, &n.Attempts, &n.LastError, &n.NextAttempt, &n.CreatedAt, &n.SentAt,
	)
	if err != nil {
		return nil, err
	}
	return n, nil
}

func (r *notificationRepository) ListDue(ctx context.Context, limit int) ([]*domain.Notification, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := r.db.Query(ctx, `SELECT `+notificationCols+`
		FROM notifications
		WHERE status = 'pending' AND next_attempt <= NOW()
		ORDER BY created_at ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*domain.Notification, 0)
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *notificationRepository) MarkSent(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE notifications
		SET status = 'sent', sent_at = NOW()
		WHERE id = $1
	`, id)
	return err
}

func (r *notificationRepository) MarkFailed(ctx context.Context, id uuid.UUID, errMsg string, nextAttempt time.Time, dlq bool) error {
	status := "pending"
	if dlq {
		status = "dlq"
	}
	_, err := r.db.Exec(ctx, `
		UPDATE notifications
		SET attempts = attempts + 1,
		    last_error = $2,
		    next_attempt = $3,
		    status = $4
		WHERE id = $1
	`, id, errMsg, nextAttempt, status)
	return err
}

func (r *notificationRepository) ListByStatus(ctx context.Context, status domain.NotificationStatus, limit int) ([]*domain.Notification, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.db.Query(ctx, `SELECT `+notificationCols+`
		FROM notifications
		WHERE status = $1
		ORDER BY created_at DESC
		LIMIT $2`, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*domain.Notification, 0)
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
