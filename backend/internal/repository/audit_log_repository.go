package repository

import (
	"context"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type AuditLogRepository interface {
	Create(ctx context.Context, audit *domain.AuditLog) error
	ListByEntity(ctx context.Context, entity string, entityID, orgID uuid.UUID) ([]*domain.AuditLog, error)
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
