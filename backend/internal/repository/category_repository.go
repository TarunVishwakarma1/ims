package repository

import (
	"context"
	"errors"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type CategoryRepository interface {
	Create(ctx context.Context, category *domain.Category) error
	GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*domain.Category, error)
	Update(ctx context.Context, category *domain.Category) error
	Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error
	List(ctx context.Context, orgID uuid.UUID) ([]*domain.Category, error)
	WithTx(tx pgx.Tx) CategoryRepository
}

type categoryRepository struct {
	db DBTX
}

func NewCategoryRepository(db DBTX) CategoryRepository {
	return &categoryRepository{db: db}
}

func (r *categoryRepository) WithTx(tx pgx.Tx) CategoryRepository {
	return &categoryRepository{db: tx}
}

func (r *categoryRepository) Create(ctx context.Context, category *domain.Category) error {
	query := `
		INSERT INTO categories (id, org_id, name, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.Exec(ctx, query, category.ID, category.OrgID, category.Name, category.Description, category.CreatedAt, category.UpdatedAt)
	return err
}

func (r *categoryRepository) GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*domain.Category, error) {
	query := `
		SELECT id, org_id, name, description, created_at, updated_at
		FROM categories
		WHERE id = $1 AND org_id = $2
	`
	category := &domain.Category{}
	err := r.db.QueryRow(ctx, query, id, orgID).Scan(&category.ID, &category.OrgID, &category.Name, &category.Description, &category.CreatedAt, &category.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return category, nil
}

func (r *categoryRepository) Update(ctx context.Context, category *domain.Category) error {
	query := `
		UPDATE categories
		SET name = $3, description = $4, updated_at = $5
		WHERE id = $1 AND org_id = $2
	`
	_, err := r.db.Exec(ctx, query, category.ID, category.OrgID, category.Name, category.Description, category.UpdatedAt)
	return err
}

func (r *categoryRepository) Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error {
	query := `
		DELETE FROM categories
		WHERE id = $1 AND org_id = $2
	`
	_, err := r.db.Exec(ctx, query, id, orgID)
	return err
}

func (r *categoryRepository) List(ctx context.Context, orgID uuid.UUID) ([]*domain.Category, error) {
	query := `
		SELECT id, org_id, name, description, created_at, updated_at
		FROM categories
		WHERE org_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	categories := make([]*domain.Category, 0)
	for rows.Next() {
		category := &domain.Category{}
		err := rows.Scan(&category.ID, &category.OrgID, &category.Name, &category.Description, &category.CreatedAt, &category.UpdatedAt)
		if err != nil {
			return nil, err
		}
		categories = append(categories, category)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return categories, nil
}
