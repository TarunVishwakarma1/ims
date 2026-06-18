package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/pkg/crypto"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type PaymentRepository interface {
	Create(ctx context.Context, p *domain.Payment) error
	GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*domain.Payment, error)
	GetByRazorpayOrderID(ctx context.Context, rzpOrderID string) (*domain.Payment, error)
	GetByRazorpayPaymentID(ctx context.Context, rzpPaymentID string) (*domain.Payment, error)
	GetByOrderID(ctx context.Context, orderID uuid.UUID, orgID uuid.UUID) (*domain.Payment, error)
	// ListByStatus pages through payments with a given status across all
	// orgs. Used by the reconciliation cron — small batches at a time.
	ListByStatus(ctx context.Context, status string, limit int, afterCreatedAt time.Time) ([]*domain.Payment, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status, method, paymentID, failureReason string, raw json.RawMessage) error
	// IncrementRefunded bumps amount_refunded by delta and (atomically) sets
	// status. Used after a refund webhook lands.
	IncrementRefunded(ctx context.Context, id uuid.UUID, delta int64, newStatus string) error
	List(ctx context.Context, orgID uuid.UUID) ([]*domain.Payment, error)
	WithTx(tx pgx.Tx) PaymentRepository

	// Refund history.
	CreateRefund(ctx context.Context, r *domain.Refund) error
	GetRefundByRazorpayID(ctx context.Context, rzpRefundID string) (*domain.Refund, error)
	ListRefunds(ctx context.Context, paymentID, orgID uuid.UUID) ([]*domain.Refund, error)
}

type paymentRepository struct {
	db  DBTX
	enc *crypto.Encryptor
}

func NewPaymentRepository(db DBTX, enc *crypto.Encryptor) PaymentRepository {
	return &paymentRepository{db: db, enc: enc}
}

func (r *paymentRepository) WithTx(tx pgx.Tx) PaymentRepository {
	return &paymentRepository{db: tx, enc: r.enc}
}

func (r *paymentRepository) Create(ctx context.Context, p *domain.Payment) error {
	var encrypted []byte
	if len(p.RawPayload) > 0 && r.enc != nil {
		var err error
		encrypted, err = r.enc.Encrypt(p.RawPayload)
		if err != nil {
			return err
		}
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO payments (id, org_id, order_id, razorpay_order_id, amount, currency, status, is_mock, raw_payload_enc, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, p.ID, p.OrgID, p.OrderID, p.RazorpayOrderID, p.Amount, p.Currency, p.Status, p.IsMock, encrypted, p.CreatedAt, p.UpdatedAt)
	return err
}

func (r *paymentRepository) scan(scanner interface {
	Scan(dest ...any) error
}) (*domain.Payment, error) {
	p := &domain.Payment{}
	var plainPayload json.RawMessage
	var encPayload []byte
	err := scanner.Scan(
		&p.ID, &p.OrgID, &p.OrderID, &p.RazorpayOrderID, &p.RazorpayPaymentID,
		&p.Amount, &p.AmountRefunded, &p.Currency, &p.Status, &p.Method, &p.FailureReason,
		&plainPayload, &encPayload, &p.IsMock, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	// Prefer encrypted column when present; fall back to legacy plaintext.
	if len(encPayload) > 0 && r.enc != nil {
		dec, decErr := r.enc.Decrypt(encPayload)
		if decErr != nil {
			return nil, decErr
		}
		p.RawPayload = dec
	} else if len(plainPayload) > 0 {
		p.RawPayload = plainPayload
	}
	_ = time.Now // placate import block (used elsewhere in repo)
	return p, nil
}

const paymentCols = `id, org_id, order_id, razorpay_order_id, razorpay_payment_id,
	amount, amount_refunded, currency, status, method, failure_reason, raw_payload, raw_payload_enc,
	is_mock, created_at, updated_at`

func (r *paymentRepository) GetByID(ctx context.Context, id, orgID uuid.UUID) (*domain.Payment, error) {
	row := r.db.QueryRow(ctx, `SELECT `+paymentCols+` FROM payments WHERE id = $1 AND org_id = $2`, id, orgID)
	p, err := r.scan(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

func (r *paymentRepository) GetByRazorpayOrderID(ctx context.Context, rzpOrderID string) (*domain.Payment, error) {
	row := r.db.QueryRow(ctx, `SELECT `+paymentCols+` FROM payments WHERE razorpay_order_id = $1`, rzpOrderID)
	p, err := r.scan(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

func (r *paymentRepository) GetByRazorpayPaymentID(ctx context.Context, rzpPaymentID string) (*domain.Payment, error) {
	row := r.db.QueryRow(ctx, `SELECT `+paymentCols+` FROM payments WHERE razorpay_payment_id = $1`, rzpPaymentID)
	p, err := r.scan(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

func (r *paymentRepository) GetByOrderID(ctx context.Context, orderID, orgID uuid.UUID) (*domain.Payment, error) {
	row := r.db.QueryRow(ctx, `SELECT `+paymentCols+` FROM payments WHERE order_id = $1 AND org_id = $2 ORDER BY created_at DESC LIMIT 1`, orderID, orgID)
	p, err := r.scan(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

func (r *paymentRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status, method, paymentID, failureReason string, raw json.RawMessage) error {
	var encrypted []byte
	if len(raw) > 0 && r.enc != nil {
		var err error
		encrypted, err = r.enc.Encrypt(raw)
		if err != nil {
			return err
		}
	}
	_, err := r.db.Exec(ctx, `
		UPDATE payments SET
			status = $2,
			method = NULLIF($3, ''),
			razorpay_payment_id = NULLIF($4, ''),
			failure_reason = NULLIF($5, ''),
			raw_payload_enc = COALESCE($6, raw_payload_enc),
			updated_at = NOW()
		WHERE id = $1
	`, id, status, method, paymentID, failureReason, encrypted)
	return err
}

func (r *paymentRepository) List(ctx context.Context, orgID uuid.UUID) ([]*domain.Payment, error) {
	rows, err := r.db.Query(ctx, `SELECT `+paymentCols+` FROM payments WHERE org_id = $1 ORDER BY created_at DESC LIMIT 500`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*domain.Payment, 0)
	for rows.Next() {
		p, err := r.scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListByStatus pages through payments with a given status across orgs. Used
// by the daily reconciliation cron to catch payments whose state may have
// drifted from the canonical RazorPay record (typically due to a missed
// webhook). `afterCreatedAt` is an exclusive cursor for batch pagination.
func (r *paymentRepository) ListByStatus(ctx context.Context, status string, limit int, afterCreatedAt time.Time) ([]*domain.Payment, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := r.db.Query(ctx, `SELECT `+paymentCols+` FROM payments
		WHERE status = $1 AND created_at > $2
		ORDER BY created_at ASC LIMIT $3`, status, afterCreatedAt, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.Payment
	for rows.Next() {
		p, err := r.scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ── Webhook events ───────────────────────────────────────────────────────

type WebhookRepository interface {
	Insert(ctx context.Context, evt *domain.WebhookEvent) error
	MarkProcessed(ctx context.Context, id uuid.UUID) error
	MarkFailed(ctx context.Context, id uuid.UUID, errMsg string, nextRetry *time.Time) error
	SendToDLQ(ctx context.Context, id uuid.UUID, errMsg string) error
	Exists(ctx context.Context, eventID string) (bool, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.WebhookEvent, []byte, error)
	IncrementAttempts(ctx context.Context, id uuid.UUID) (int, error)
	// ListDLQ returns failed events parked in the dead-letter queue.
	ListDLQ(ctx context.Context, limit int) ([]*domain.WebhookEvent, error)
	// ListRecent returns the most recent N webhook events regardless of
	// status. Drives the success-log admin view.
	ListRecent(ctx context.Context, limit int, status string) ([]*domain.WebhookEvent, error)
}

type webhookRepository struct {
	db  DBTX
	enc *crypto.Encryptor
}

func NewWebhookRepository(db DBTX, enc *crypto.Encryptor) WebhookRepository {
	return &webhookRepository{db: db, enc: enc}
}

func (r *webhookRepository) Insert(ctx context.Context, evt *domain.WebhookEvent) error {
	var encrypted []byte
	if len(evt.Payload) > 0 && r.enc != nil {
		var err error
		encrypted, err = r.enc.Encrypt(evt.Payload)
		if err != nil {
			return err
		}
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO webhook_events (id, provider, event_id, event_type, signature, payload, payload_enc, status, created_at)
		VALUES ($1, $2, $3, $4, $5, NULL, $6, $7, $8)
	`, evt.ID, evt.Provider, evt.EventID, evt.EventType, evt.Signature, encrypted, evt.Status, evt.CreatedAt)
	return err
}

func (r *webhookRepository) MarkProcessed(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE webhook_events SET status = 'processed', processed_at = NOW() WHERE id = $1`, id)
	return err
}

func (r *webhookRepository) MarkFailed(ctx context.Context, id uuid.UUID, errMsg string, nextRetry *time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE webhook_events SET
			status = 'failed',
			error = $2,
			next_retry_at = $3,
			processed_at = NOW()
		WHERE id = $1
	`, id, errMsg, nextRetry)
	return err
}

func (r *webhookRepository) SendToDLQ(ctx context.Context, id uuid.UUID, errMsg string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE webhook_events SET status = 'failed', dlq = true, error = $2, next_retry_at = NULL, processed_at = NOW()
		WHERE id = $1
	`, id, errMsg)
	return err
}

func (r *webhookRepository) Exists(ctx context.Context, eventID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM webhook_events WHERE event_id = $1)`, eventID).Scan(&exists)
	return exists, err
}

func (r *webhookRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.WebhookEvent, []byte, error) {
	var evt domain.WebhookEvent
	var encPayload []byte
	err := r.db.QueryRow(ctx, `
		SELECT id, provider, event_id, event_type, signature, status, error, payload_enc, created_at, processed_at
		FROM webhook_events WHERE id = $1
	`, id).Scan(&evt.ID, &evt.Provider, &evt.EventID, &evt.EventType, &evt.Signature, &evt.Status, &evt.Error, &encPayload, &evt.CreatedAt, &evt.ProcessedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, domain.ErrNotFound
		}
		return nil, nil, err
	}
	var raw []byte
	if len(encPayload) > 0 && r.enc != nil {
		raw, err = r.enc.Decrypt(encPayload)
		if err != nil {
			return nil, nil, err
		}
	}
	evt.Payload = raw
	return &evt, raw, nil
}

func (r *webhookRepository) IncrementAttempts(ctx context.Context, id uuid.UUID) (int, error) {
	var attempts int
	err := r.db.QueryRow(ctx, `
		UPDATE webhook_events SET attempts = attempts + 1 WHERE id = $1 RETURNING attempts
	`, id).Scan(&attempts)
	return attempts, err
}

// ListRecent returns the most recent webhook events. `status` is optional
// (e.g. "processed" filters to success log; "" matches all).
func (r *webhookRepository) ListRecent(ctx context.Context, limit int, status string) ([]*domain.WebhookEvent, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	var rows pgx.Rows
	var err error
	if status != "" {
		rows, err = r.db.Query(ctx, `
			SELECT id, provider, event_id, event_type, status, error, attempts, created_at, processed_at
			FROM webhook_events
			WHERE status = $1
			ORDER BY created_at DESC LIMIT $2
		`, status, limit)
	} else {
		rows, err = r.db.Query(ctx, `
			SELECT id, provider, event_id, event_type, status, error, attempts, created_at, processed_at
			FROM webhook_events
			ORDER BY created_at DESC LIMIT $1
		`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*domain.WebhookEvent, 0)
	for rows.Next() {
		var e domain.WebhookEvent
		var attempts int
		if err := rows.Scan(&e.ID, &e.Provider, &e.EventID, &e.EventType, &e.Status, &e.Error, &attempts, &e.CreatedAt, &e.ProcessedAt); err != nil {
			return nil, err
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}

func (r *webhookRepository) ListDLQ(ctx context.Context, limit int) ([]*domain.WebhookEvent, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, provider, event_id, event_type, status, error, attempts, created_at, processed_at
		FROM webhook_events WHERE dlq = true ORDER BY created_at DESC LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*domain.WebhookEvent, 0)
	for rows.Next() {
		var e domain.WebhookEvent
		var attempts int
		if err := rows.Scan(&e.ID, &e.Provider, &e.EventID, &e.EventType, &e.Status, &e.Error, &attempts, &e.CreatedAt, &e.ProcessedAt); err != nil {
			return nil, err
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}

// IncrementRefunded bumps amount_refunded by `delta` and flips status in a
// single UPDATE. Caller decides newStatus (`partially_refunded` while the
// sum is below amount; `refunded` when fully refunded). delta is paise.
func (r *paymentRepository) IncrementRefunded(ctx context.Context, id uuid.UUID, delta int64, newStatus string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE payments
		SET amount_refunded = amount_refunded + $2,
		    status          = $3,
		    updated_at      = NOW()
		WHERE id = $1
	`, id, delta, newStatus)
	return err
}

// CreateRefund inserts a refund row.
func (r *paymentRepository) CreateRefund(ctx context.Context, rf *domain.Refund) error {
	if rf.ID == uuid.Nil {
		rf.ID = uuid.New()
	}
	if rf.CreatedAt.IsZero() {
		rf.CreatedAt = time.Now().UTC()
	}
	if rf.Status == "" {
		rf.Status = "processed"
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO refunds (id, payment_id, org_id, amount, razorpay_refund_id, status, reason, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, rf.ID, rf.PaymentID, rf.OrgID, rf.Amount, rf.RazorpayRefundID, rf.Status, rf.Reason, rf.CreatedAt)
	return err
}

func (r *paymentRepository) GetRefundByRazorpayID(ctx context.Context, rzpRefundID string) (*domain.Refund, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, payment_id, org_id, amount, razorpay_refund_id, status, reason, created_at
		FROM refunds WHERE razorpay_refund_id = $1
	`, rzpRefundID)
	rf := &domain.Refund{}
	err := row.Scan(&rf.ID, &rf.PaymentID, &rf.OrgID, &rf.Amount, &rf.RazorpayRefundID, &rf.Status, &rf.Reason, &rf.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return rf, nil
}

func (r *paymentRepository) ListRefunds(ctx context.Context, paymentID, orgID uuid.UUID) ([]*domain.Refund, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, payment_id, org_id, amount, razorpay_refund_id, status, reason, created_at
		FROM refunds
		WHERE payment_id = $1 AND org_id = $2
		ORDER BY created_at DESC
	`, paymentID, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*domain.Refund, 0)
	for rows.Next() {
		rf := &domain.Refund{}
		if err := rows.Scan(&rf.ID, &rf.PaymentID, &rf.OrgID, &rf.Amount, &rf.RazorpayRefundID, &rf.Status, &rf.Reason, &rf.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, rf)
	}
	return out, rows.Err()
}
