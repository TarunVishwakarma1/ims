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

// orderCols — single source of truth for the column list. Every scan goes
// through scanOrder so adding a column means changing ONE place.
const orderCols = `id, org_id, user_id, status, total_amount, created_at, updated_at,
	order_type, buyer_org_id, supplier_org_id, supplier_location_id, customer_id,
	delivery_address_id, subtotal, delivery_fee, discount,
	payment_status, payment_id,
	accepted_at, shipped_at, delivered_at, completed_at, cancelled_at`

func scanOrder(scanner interface {
	Scan(dest ...any) error
}) (*domain.Order, error) {
	o := &domain.Order{}
	err := scanner.Scan(
		&o.ID, &o.OrgID, &o.UserID, &o.Status, &o.TotalAmount, &o.CreatedAt, &o.UpdatedAt,
		&o.OrderType, &o.BuyerOrgID, &o.SupplierOrgID, &o.SupplierLocationID, &o.CustomerID,
		&o.DeliveryAddressID, &o.Subtotal, &o.DeliveryFee, &o.Discount,
		&o.PaymentStatus, &o.PaymentID,
		&o.AcceptedAt, &o.ShippedAt, &o.DeliveredAt, &o.CompletedAt, &o.CancelledAt,
	)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (r *orderRepository) Create(ctx context.Context, order *domain.Order) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO orders (
			id, org_id, user_id, status, total_amount, created_at, updated_at,
			order_type, buyer_org_id, supplier_org_id, supplier_location_id, customer_id,
			delivery_address_id, subtotal, delivery_fee, discount, payment_status, payment_id
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17, $18
		)
	`,
		order.ID, order.OrgID, order.UserID, order.Status, order.TotalAmount, order.CreatedAt, order.UpdatedAt,
		order.OrderType, order.BuyerOrgID, order.SupplierOrgID, order.SupplierLocationID, order.CustomerID,
		order.DeliveryAddressID, order.Subtotal, order.DeliveryFee, order.Discount,
		nullIfEmpty(order.PaymentStatus, "unpaid"), order.PaymentID,
	)
	return err
}

func (r *orderRepository) GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*domain.Order, error) {
	row := r.db.QueryRow(ctx, `SELECT `+orderCols+` FROM orders WHERE id = $1 AND org_id = $2`, id, orgID)
	o, err := scanOrder(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return o, nil
}

func (r *orderRepository) Update(ctx context.Context, order *domain.Order) error {
	_, err := r.db.Exec(ctx, `
		UPDATE orders SET
			status = $3, total_amount = $4, subtotal = $5,
			payment_status = $6, payment_id = $7,
			accepted_at = $8, shipped_at = $9, delivered_at = $10,
			completed_at = $11, cancelled_at = $12,
			updated_at = $13
		WHERE id = $1 AND org_id = $2
	`,
		order.ID, order.OrgID, order.Status, order.TotalAmount, order.Subtotal,
		nullIfEmpty(order.PaymentStatus, "unpaid"), order.PaymentID,
		order.AcceptedAt, order.ShippedAt, order.DeliveredAt,
		order.CompletedAt, order.CancelledAt,
		order.UpdatedAt,
	)
	return err
}

func nullIfEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func (r *orderRepository) Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM orders WHERE id = $1 AND org_id = $2`, id, orgID)
	return err
}

func (r *orderRepository) List(ctx context.Context, orgID uuid.UUID) ([]*domain.Order, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+orderCols+` FROM orders WHERE org_id = $1 ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*domain.Order, 0)
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, o)
	}
	return list, rows.Err()
}

func (r *orderRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.OrderStatus, orgID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE orders SET status = $3, updated_at = NOW() WHERE id = $1 AND org_id = $2`, id, orgID, status)
	return err
}

func (r *orderRepository) ListByUser(ctx context.Context, userID uuid.UUID, orgID uuid.UUID) ([]*domain.Order, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+orderCols+` FROM orders WHERE user_id = $1 AND org_id = $2 ORDER BY created_at DESC`,
		userID, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*domain.Order, 0)
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, o)
	}
	return list, rows.Err()
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
