package repository

import (
	"context"
	"errors"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ProductRepository interface {
	Create(ctx context.Context, product *domain.Product) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Product, error)
	GetBySKU(ctx context.Context, sku string) (*domain.Product, error)
	Update(ctx context.Context, product *domain.Product) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context) ([]*domain.Product, error)
	ListByCategory(ctx context.Context, categoryID uuid.UUID) ([]*domain.Product, error)
	WithTx(tx pgx.Tx) ProductRepository
}

type productRepository struct {
	db DBTX
}

func NewProductRepository(db DBTX) ProductRepository {
	return &productRepository{db: db}
}

func (r *productRepository) WithTx(tx pgx.Tx) ProductRepository {
	return &productRepository{db: tx}
}

func (r *productRepository) Create(ctx context.Context, product *domain.Product) error {
	query := `
		INSERT INTO products (id, category_id, name, description, sku, price, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.Exec(ctx, query, product.ID, product.CategoryID, product.Name, product.Description, product.SKU, product.Price, product.CreatedAt, product.UpdatedAt)
	return err
}

func (r *productRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Product, error) {
	query := `
		SELECT id, category_id, name, description, sku, price, created_at, updated_at
		FROM products
		WHERE id = $1
	`
	product := &domain.Product{}
	err := r.db.QueryRow(ctx, query, id).Scan(&product.ID, &product.CategoryID, &product.Name, &product.Description, &product.SKU, &product.Price, &product.CreatedAt, &product.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return product, nil
}

func (r *productRepository) GetBySKU(ctx context.Context, sku string) (*domain.Product, error) {
	query := `
		SELECT id, category_id, name, description, sku, price, created_at, updated_at
		FROM products
		WHERE sku = $1
	`
	product := &domain.Product{}
	err := r.db.QueryRow(ctx, query, sku).Scan(&product.ID, &product.CategoryID, &product.Name, &product.Description, &product.SKU, &product.Price, &product.CreatedAt, &product.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return product, nil
}

func (r *productRepository) Update(ctx context.Context, product *domain.Product) error {
	query := `
		UPDATE products
		SET category_id = $2, name = $3, description = $4, sku = $5, price = $6, updated_at = $7
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query, product.ID, product.CategoryID, product.Name, product.Description, product.SKU, product.Price, product.UpdatedAt)
	return err
}

func (r *productRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `
		DELETE FROM products
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query, id)
	return err
}

func (r *productRepository) List(ctx context.Context) ([]*domain.Product, error) {
	query := `
		SELECT id, category_id, name, description, sku, price, created_at, updated_at
		FROM products
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []*domain.Product
	for rows.Next() {
		product := &domain.Product{}
		err := rows.Scan(&product.ID, &product.CategoryID, &product.Name, &product.Description, &product.SKU, &product.Price, &product.CreatedAt, &product.UpdatedAt)
		if err != nil {
			return nil, err
		}
		products = append(products, product)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return products, nil
}

func (r *productRepository) ListByCategory(ctx context.Context, categoryID uuid.UUID) ([]*domain.Product, error) {
	query := `
		SELECT id, category_id, name, description, sku, price, created_at, updated_at
		FROM products
		WHERE category_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, categoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []*domain.Product
	for rows.Next() {
		product := &domain.Product{}
		err := rows.Scan(&product.ID, &product.CategoryID, &product.Name, &product.Description, &product.SKU, &product.Price, &product.CreatedAt, &product.UpdatedAt)
		if err != nil {
			return nil, err
		}
		products = append(products, product)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return products, nil
}
