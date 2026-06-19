package repository

import (
	"context"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type organizationRepository struct {
	db DBTX
}

func NewOrganizationRepository(db DBTX) domain.OrganizationRepository {
	return &organizationRepository{db: db}
}

func (r *organizationRepository) WithTx(tx pgx.Tx) domain.OrganizationRepository {
	return &organizationRepository{db: tx}
}

func (r *organizationRepository) Create(ctx context.Context, org *domain.Organization) error {
	query := `
		INSERT INTO organizations (id, name, slug, plan_type, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.Exec(ctx, query,
		org.ID, org.Name, org.Slug, org.PlanType, org.IsActive, org.CreatedAt, org.UpdatedAt,
	)
	return err
}

func (r *organizationRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Organization, error) {
	query := `
		SELECT id, name, slug, plan_type, is_active, created_at, updated_at
		FROM organizations
		WHERE id = $1
	`
	org := &domain.Organization{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&org.ID, &org.Name, &org.Slug, &org.PlanType, &org.IsActive, &org.CreatedAt, &org.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	return org, err
}

func (r *organizationRepository) GetBySlug(ctx context.Context, slug string) (*domain.Organization, error) {
	query := `
		SELECT id, name, slug, plan_type, is_active, created_at, updated_at
		FROM organizations
		WHERE slug = $1
	`
	org := &domain.Organization{}
	err := r.db.QueryRow(ctx, query, slug).Scan(
		&org.ID, &org.Name, &org.Slug, &org.PlanType, &org.IsActive, &org.CreatedAt, &org.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	return org, err
}
