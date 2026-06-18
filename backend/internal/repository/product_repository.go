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
	GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*domain.Product, error)
	GetBySKU(ctx context.Context, sku string, orgID uuid.UUID) (*domain.Product, error)
	Update(ctx context.Context, product *domain.Product) error
	Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error
	List(ctx context.Context, orgID uuid.UUID) ([]*domain.Product, error)
	ListByCategory(ctx context.Context, categoryID uuid.UUID, orgID uuid.UUID) ([]*domain.Product, error)
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

const productCols = `id, org_id, category_id, name, description, sku, price, gst_rate, created_at, updated_at`

func scanProduct(scanner interface {
	Scan(dest ...any) error
}) (*domain.Product, error) {
	p := &domain.Product{}
	err := scanner.Scan(
		&p.ID, &p.OrgID, &p.CategoryID, &p.Name, &p.Description, &p.SKU,
		&p.Price, &p.GSTRate, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *productRepository) Create(ctx context.Context, product *domain.Product) error {
	query := `
		INSERT INTO products (id, org_id, category_id, name, description, sku, price, gst_rate, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.Exec(ctx, query,
		product.ID, product.OrgID, product.CategoryID, product.Name, product.Description,
		product.SKU, product.Price, product.GSTRate, product.CreatedAt, product.UpdatedAt,
	)
	return err
}

func (r *productRepository) GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*domain.Product, error) {
	row := r.db.QueryRow(ctx, `SELECT `+productCols+` FROM products WHERE id = $1 AND org_id = $2`, id, orgID)
	p, err := scanProduct(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

func (r *productRepository) GetBySKU(ctx context.Context, sku string, orgID uuid.UUID) (*domain.Product, error) {
	row := r.db.QueryRow(ctx, `SELECT `+productCols+` FROM products WHERE sku = $1 AND org_id = $2`, sku, orgID)
	p, err := scanProduct(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

func (r *productRepository) Update(ctx context.Context, product *domain.Product) error {
	query := `
		UPDATE products
		SET category_id = $3, name = $4, description = $5, sku = $6, price = $7, gst_rate = $8, updated_at = $9
		WHERE id = $1 AND org_id = $2
	`
	_, err := r.db.Exec(ctx, query,
		product.ID, product.OrgID, product.CategoryID, product.Name, product.Description,
		product.SKU, product.Price, product.GSTRate, product.UpdatedAt,
	)
	return err
}

func (r *productRepository) Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM products WHERE id = $1 AND org_id = $2`, id, orgID)
	return err
}

func (r *productRepository) List(ctx context.Context, orgID uuid.UUID) ([]*domain.Product, error) {
	rows, err := r.db.Query(ctx, `SELECT `+productCols+` FROM products WHERE org_id = $1 ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	products := make([]*domain.Product, 0)
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, rows.Err()
}

func (r *productRepository) ListByCategory(ctx context.Context, categoryID uuid.UUID, orgID uuid.UUID) ([]*domain.Product, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+productCols+` FROM products WHERE category_id = $1 AND org_id = $2 ORDER BY created_at DESC`,
		categoryID, orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	products := make([]*domain.Product, 0)
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, rows.Err()
}
