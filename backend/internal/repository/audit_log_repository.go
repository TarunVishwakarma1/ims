package repository

import (
	"context"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/jackc/pgx/v5"
)

type AuditLogRepository interface {
	Create(ctx context.Context, audit *domain.AuditLog) error
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
