package repository

import (
	"context"
	"errors"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OrderRepository interface {
	Create(ctx context.Context, order *domain.Order) error
	GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*domain.Order, error)
	Update(ctx context.Context, order *domain.Order) error
	Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error
	List(ctx context.Context, orgID uuid.UUID) ([]*domain.Order, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.OrderStatus, orgID uuid.UUID) error
	ListByUser(ctx context.Context, userID uuid.UUID, orgID uuid.UUID) ([]*domain.Order, error)
	CreateOrderItem(ctx context.Context, item *domain.OrderItem) error
	GetOrderItems(ctx context.Context, orderID uuid.UUID, orgID uuid.UUID) ([]*domain.OrderItem, error)
	WithTx(tx pgx.Tx) OrderRepository
	BeginTx(ctx context.Context) (pgx.Tx, error)
}

type orderRepository struct {
	db   DBTX
	pool *pgxpool.Pool
}

func NewOrderRepository(pool *pgxpool.Pool) OrderRepository {
	return &orderRepository{db: pool, pool: pool}
}

func (r *orderRepository) WithTx(tx pgx.Tx) OrderRepository {
	return &orderRepository{db: tx, pool: r.pool}
}

func (r *orderRepository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	if r.pool == nil {
		return nil, errors.New("cannot begin transaction: no pool available")
	}
	return r.pool.Begin(ctx)
}

func (r *orderRepository) Create(ctx context.Context, order *domain.Order) error {
	query := `
		INSERT INTO orders (id, org_id, user_id, status, total_amount, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.Exec(ctx, query, order.ID, order.OrgID, order.UserID, order.Status, order.TotalAmount, order.CreatedAt, order.UpdatedAt)
	return err
}

func (r *orderRepository) GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*domain.Order, error) {
	query := `
		SELECT id, org_id, user_id, status, total_amount, created_at, updated_at
		FROM orders
		WHERE id = $1 AND org_id = $2
	`
	order := &domain.Order{}
	err := r.db.QueryRow(ctx, query, id, orgID).Scan(&order.ID, &order.OrgID, &order.UserID, &order.Status, &order.TotalAmount, &order.CreatedAt, &order.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return order, nil
}

func (r *orderRepository) Update(ctx context.Context, order *domain.Order) error {
	query := `
		UPDATE orders
		SET user_id = $3, status = $4, total_amount = $5, updated_at = $6
		WHERE id = $1 AND org_id = $2
	`
	_, err := r.db.Exec(ctx, query, order.ID, order.OrgID, order.UserID, order.Status, order.TotalAmount, order.UpdatedAt)
	return err
}

func (r *orderRepository) Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error {
	query := `
		DELETE FROM orders
		WHERE id = $1 AND org_id = $2
	`
	_, err := r.db.Exec(ctx, query, id, orgID)
	return err
}

func (r *orderRepository) List(ctx context.Context, orgID uuid.UUID) ([]*domain.Order, error) {
	query := `
		SELECT id, org_id, user_id, status, total_amount, created_at, updated_at
		FROM orders
		WHERE org_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*domain.Order, 0)
	for rows.Next() {
		order := &domain.Order{}
		err := rows.Scan(&order.ID, &order.OrgID, &order.UserID, &order.Status, &order.TotalAmount, &order.CreatedAt, &order.UpdatedAt)
		if err != nil {
			return nil, err
		}
		list = append(list, order)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return list, nil
}

func (r *orderRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.OrderStatus, orgID uuid.UUID) error {
	query := `
		UPDATE orders
		SET status = $3, updated_at = NOW()
		WHERE id = $1 AND org_id = $2
	`
	_, err := r.db.Exec(ctx, query, id, orgID, status)
	return err
}

func (r *orderRepository) ListByUser(ctx context.Context, userID uuid.UUID, orgID uuid.UUID) ([]*domain.Order, error) {
	query := `
		SELECT id, org_id, user_id, status, total_amount, created_at, updated_at
		FROM orders
		WHERE user_id = $1 AND org_id = $2
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, userID, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*domain.Order, 0)
	for rows.Next() {
		order := &domain.Order{}
		err := rows.Scan(&order.ID, &order.OrgID, &order.UserID, &order.Status, &order.TotalAmount, &order.CreatedAt, &order.UpdatedAt)
		if err != nil {
			return nil, err
		}
		list = append(list, order)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return list, nil
}

func (r *orderRepository) CreateOrderItem(ctx context.Context, item *domain.OrderItem) error {
	query := `
		INSERT INTO order_items (id, org_id, order_id, product_id, quantity, unit_price)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.Exec(ctx, query, item.ID, item.OrgID, item.OrderID, item.ProductID, item.Quantity, item.UnitPrice)
	return err
}

func (r *orderRepository) GetOrderItems(ctx context.Context, orderID uuid.UUID, orgID uuid.UUID) ([]*domain.OrderItem, error) {
	query := `
		SELECT id, org_id, order_id, product_id, quantity, unit_price
		FROM order_items
		WHERE order_id = $1 AND org_id = $2
	`
	rows, err := r.db.Query(ctx, query, orderID, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*domain.OrderItem, 0)
	for rows.Next() {
		item := &domain.OrderItem{}
		err := rows.Scan(&item.ID, &item.OrgID, &item.OrderID, &item.ProductID, &item.Quantity, &item.UnitPrice)
		if err != nil {
			return nil, err
		}
		list = append(list, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return list, nil
}
