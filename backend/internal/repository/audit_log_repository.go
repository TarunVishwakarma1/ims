package repository

import (
	"context"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditLogRepository interface {
	Create(ctx context.Context, audit *domain.AuditLog) error
}

type auditLogRepository struct {
	pool *pgxpool.Pool
}

func NewAuditLogRepository(pool *pgxpool.Pool) AuditLogRepository {
	return &auditLogRepository{pool: pool}
}

func (r *auditLogRepository) Create(ctx context.Context, audit *domain.AuditLog) error {
	query := `
		INSERT INTO audit_logs (id, user_id, action, entity, entity_id, ip_address, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.pool.Exec(ctx, query, audit.ID, audit.UserID, audit.Action, audit.Entity, audit.EntityID, audit.IPAddress, audit.CreatedAt)
	return err
}
