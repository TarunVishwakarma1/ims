package repository

import (
	"context"
	"errors"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type InventoryRepository interface {
	Create(ctx context.Context, inventory *domain.Inventory) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Inventory, error)
	GetByProductID(ctx context.Context, productID uuid.UUID) (*domain.Inventory, error)
	Update(ctx context.Context, inventory *domain.Inventory) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context) ([]*domain.Inventory, error)
	ListLowStock(ctx context.Context) ([]*domain.Inventory, error)
	WithTx(tx pgx.Tx) InventoryRepository
}

type inventoryRepository struct {
	db DBTX
}

func NewInventoryRepository(db DBTX) InventoryRepository {
	return &inventoryRepository{db: db}
}

func (r *inventoryRepository) WithTx(tx pgx.Tx) InventoryRepository {
	return &inventoryRepository{db: tx}
}

func (r *inventoryRepository) Create(ctx context.Context, inventory *domain.Inventory) error {
	query := `
		INSERT INTO inventory (id, product_id, quantity, low_stock_threshold, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.db.Exec(ctx, query, inventory.ID, inventory.ProductID, inventory.Quantity, inventory.LowStockThreshold, inventory.UpdatedAt)
	return err
}

func (r *inventoryRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Inventory, error) {
	query := `
		SELECT id, product_id, quantity, low_stock_threshold, updated_at
		FROM inventory
		WHERE id = $1
	`
	inventory := &domain.Inventory{}
	err := r.db.QueryRow(ctx, query, id).Scan(&inventory.ID, &inventory.ProductID, &inventory.Quantity, &inventory.LowStockThreshold, &inventory.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return inventory, nil
}

func (r *inventoryRepository) GetByProductID(ctx context.Context, productID uuid.UUID) (*domain.Inventory, error) {
	query := `
		SELECT id, product_id, quantity, low_stock_threshold, updated_at
		FROM inventory
		WHERE product_id = $1
	`
	inventory := &domain.Inventory{}
	err := r.db.QueryRow(ctx, query, productID).Scan(&inventory.ID, &inventory.ProductID, &inventory.Quantity, &inventory.LowStockThreshold, &inventory.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return inventory, nil
}

func (r *inventoryRepository) Update(ctx context.Context, inventory *domain.Inventory) error {
	query := `
		UPDATE inventory
		SET product_id = $2, quantity = $3, low_stock_threshold = $4, updated_at = $5
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query, inventory.ID, inventory.ProductID, inventory.Quantity, inventory.LowStockThreshold, inventory.UpdatedAt)
	return err
}

func (r *inventoryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `
		DELETE FROM inventory
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query, id)
	return err
}

func (r *inventoryRepository) List(ctx context.Context) ([]*domain.Inventory, error) {
	query := `
		SELECT id, product_id, quantity, low_stock_threshold, updated_at
		FROM inventory
		ORDER BY updated_at DESC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*domain.Inventory, 0)
	for rows.Next() {
		inv := &domain.Inventory{}
		err := rows.Scan(&inv.ID, &inv.ProductID, &inv.Quantity, &inv.LowStockThreshold, &inv.UpdatedAt)
		if err != nil {
			return nil, err
		}
		list = append(list, inv)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return list, nil
}

func (r *inventoryRepository) ListLowStock(ctx context.Context) ([]*domain.Inventory, error) {
	query := `
		SELECT id, product_id, quantity, low_stock_threshold, updated_at
		FROM inventory
		WHERE quantity <= low_stock_threshold
		ORDER BY quantity ASC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*domain.Inventory, 0)
	for rows.Next() {
		inv := &domain.Inventory{}
		err := rows.Scan(&inv.ID, &inv.ProductID, &inv.Quantity, &inv.LowStockThreshold, &inv.UpdatedAt)
		if err != nil {
			return nil, err
		}
		list = append(list, inv)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return list, nil
}
