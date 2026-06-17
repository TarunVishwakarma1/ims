package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// AuditFilters drives the audit-log admin page. All fields optional; an
// empty struct returns the most recent rows for the org.
type AuditFilters struct {
	Entity       string     // e.g. "orders", "inventory"
	ActionPrefix string     // e.g. "order." or "payment."
	UserID       *uuid.UUID // exact match
	From         *time.Time // inclusive lower bound
	To           *time.Time // inclusive upper bound
	Page         int
	PerPage      int
}

type AuditLogRepository interface {
	Create(ctx context.Context, audit *domain.AuditLog) error
	ListByEntity(ctx context.Context, entity string, entityID, orgID uuid.UUID) ([]*domain.AuditLog, error)
	ListFiltered(ctx context.Context, orgID uuid.UUID, f AuditFilters) (items []*domain.AuditLog, total int, err error)
	WithTx(tx pgx.Tx) AuditLogRepository
}

type auditLogRepository struct {
	db DBTX
}

func NewAuditLogRepository(db DBTX) AuditLogRepository {
	return &auditLogRepository{db: db}
}

func (r *auditLogRepository) WithTx(tx pgx.Tx) AuditLogRepository {
	return &auditLogRepository{db: tx}
}

func (r *auditLogRepository) Create(ctx context.Context, audit *domain.AuditLog) error {
	query := `
		INSERT INTO audit_logs (id, org_id, user_id, action, entity, entity_id, ip_address, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.Exec(ctx, query, audit.ID, audit.OrgID, audit.UserID, audit.Action, audit.Entity, audit.EntityID, audit.IPAddress, audit.CreatedAt)
	return err
}

// ListFiltered drives the audit admin page. Builds a dynamic WHERE clause
// from the filter struct and paginates. Returns total count alongside the
// page so the UI can show "X of Y".
func (r *auditLogRepository) ListFiltered(ctx context.Context, orgID uuid.UUID, f AuditFilters) ([]*domain.AuditLog, int, error) {
	conds := []string{"org_id = $1"}
	args := []any{orgID}
	add := func(sql string, val any) {
		args = append(args, val)
		conds = append(conds, fmt.Sprintf(sql, len(args)))
	}
	if f.Entity != "" {
		add("entity = $%d", f.Entity)
	}
	if f.ActionPrefix != "" {
		add("action LIKE $%d", f.ActionPrefix+"%")
	}
	if f.UserID != nil {
		add("user_id = $%d", *f.UserID)
	}
	if f.From != nil {
		add("created_at >= $%d", *f.From)
	}
	if f.To != nil {
		add("created_at <= $%d", *f.To)
	}
	where := " WHERE " + strings.Join(conds, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	page := f.Page
	if page < 1 {
		page = 1
	}
	per := f.PerPage
	if per <= 0 {
		per = 50
	}
	if per > 500 {
		per = 500
	}
	offset := (page - 1) * per

	q := `SELECT id, org_id, user_id, action, entity, entity_id, COALESCE(ip_address, ''), created_at
		FROM audit_logs` + where + ` ORDER BY created_at DESC LIMIT ` + fmt.Sprintf("%d OFFSET %d", per, offset)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]*domain.AuditLog, 0)
	for rows.Next() {
		a := &domain.AuditLog{}
		if err := rows.Scan(&a.ID, &a.OrgID, &a.UserID, &a.Action, &a.Entity, &a.EntityID, &a.IPAddress, &a.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

// ListByEntity returns the audit trail for a single entity (e.g. one order),
// chronological ascending — feeds the UI timeline view.
func (r *auditLogRepository) ListByEntity(ctx context.Context, entity string, entityID, orgID uuid.UUID) ([]*domain.AuditLog, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, org_id, user_id, action, entity, entity_id, COALESCE(ip_address, ''), created_at
		FROM audit_logs
		WHERE entity = $1 AND entity_id = $2 AND org_id = $3
		ORDER BY created_at ASC
	`, entity, entityID, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.AuditLog
	for rows.Next() {
		a := &domain.AuditLog{}
		if err := rows.Scan(&a.ID, &a.OrgID, &a.UserID, &a.Action, &a.Entity, &a.EntityID, &a.IPAddress, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
