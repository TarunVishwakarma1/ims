package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

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
	// ListFiltered returns a paginated, filtered view. limit==0 means no
	// limit (used by export). totalCount returns the count across the
	// filtered set BEFORE pagination.
	ListFiltered(ctx context.Context, orgID uuid.UUID, f domain.OrderListFilters) (items []*domain.Order, total int, err error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.OrderStatus, orgID uuid.UUID) error
	ListByUser(ctx context.Context, userID uuid.UUID, orgID uuid.UUID) ([]*domain.Order, error)
	CreateOrderItem(ctx context.Context, item *domain.OrderItem) error
	GetOrderItems(ctx context.Context, orderID uuid.UUID, orgID uuid.UUID) ([]*domain.OrderItem, error)
	WithTx(tx pgx.Tx) OrderRepository
	BeginTx(ctx context.Context) (pgx.Tx, error)

	// AllocateInvoiceNumber atomically bumps the org's invoice sequence for
	// the given financial-year label and stamps the result on the order. No-op
	// if invoice_number is already set. Returns the allocated number.
	AllocateInvoiceNumber(ctx context.Context, orderID, orgID uuid.UUID, fyLabel string) (string, error)

	ListByCustomer(ctx context.Context, customerID uuid.UUID, cursorCreatedAt time.Time, cursorID uuid.UUID, limit int) ([]CustomerOrderRow, error)
	GetByCustomerAndID(ctx context.Context, customerID, orderID uuid.UUID) (*CustomerOrderRow, []OrderItemRow, error)
	CancelByCustomer(tx pgx.Tx, customerID, orderID uuid.UUID, newStatus string) (*CancelSnapshot, error)
	RestoreStock(ctx context.Context, tx pgx.Tx, items []OrderItemRow, orderID uuid.UUID) error
}

type CustomerOrderRow struct {
	ID                      uuid.UUID
	OrgID                   uuid.UUID
	CustomerID              uuid.UUID
	InvoiceNumber           string
	Status                  string
	PaymentStatus           string
	PaymentID               *uuid.UUID
	Subtotal                int64
	DeliveryFee             int64
	Discount                int64
	TotalAmount             int64
	DeliveryAddressSnapshot []byte
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

type OrderItemRow struct {
	ProductID    uuid.UUID
	ProductName  string
	ProductSlug  string
	ProductImage string
	Quantity     int
	UnitPrice    int64
}

type CancelSnapshot struct {
	OldStatus     string
	PaymentStatus string
	PaymentID     *uuid.UUID
	TotalAmount   int64
	OrgID         uuid.UUID
	Items         []OrderItemRow
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
	tax_amount, tax_cgst, tax_sgst, tax_igst, is_inter_state,
	payment_status, payment_id,
	accepted_at, shipped_at, delivered_at, completed_at, cancelled_at,
	invoice_number`

func scanOrder(scanner interface {
	Scan(dest ...any) error
}) (*domain.Order, error) {
	o := &domain.Order{}
	err := scanner.Scan(
		&o.ID, &o.OrgID, &o.UserID, &o.Status, &o.TotalAmount, &o.CreatedAt, &o.UpdatedAt,
		&o.OrderType, &o.BuyerOrgID, &o.SupplierOrgID, &o.SupplierLocationID, &o.CustomerID,
		&o.DeliveryAddressID, &o.Subtotal, &o.DeliveryFee, &o.Discount,
		&o.TaxAmount, &o.TaxCGST, &o.TaxSGST, &o.TaxIGST, &o.IsInterState,
		&o.PaymentStatus, &o.PaymentID,
		&o.AcceptedAt, &o.ShippedAt, &o.DeliveredAt, &o.CompletedAt, &o.CancelledAt,
		&o.InvoiceNumber,
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
			delivery_address_id, subtotal, delivery_fee, discount,
			tax_amount, tax_cgst, tax_sgst, tax_igst, is_inter_state,
			payment_status, payment_id
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12,
			$13, $14, $15, $16,
			$17, $18, $19, $20, $21,
			$22, $23
		)
	`,
		order.ID, order.OrgID, order.UserID, order.Status, order.TotalAmount, order.CreatedAt, order.UpdatedAt,
		order.OrderType, order.BuyerOrgID, order.SupplierOrgID, order.SupplierLocationID, order.CustomerID,
		order.DeliveryAddressID, order.Subtotal, order.DeliveryFee, order.Discount,
		order.TaxAmount, order.TaxCGST, order.TaxSGST, order.TaxIGST, order.IsInterState,
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
			status = $3, total_amount = $4, subtotal = $5, discount = $6,
			tax_amount = $7, tax_cgst = $8, tax_sgst = $9, tax_igst = $10, is_inter_state = $11,
			payment_status = $12, payment_id = $13,
			accepted_at = $14, shipped_at = $15, delivered_at = $16,
			completed_at = $17, cancelled_at = $18,
			updated_at = $19
		WHERE id = $1 AND org_id = $2
	`,
		order.ID, order.OrgID, order.Status, order.TotalAmount, order.Subtotal, order.Discount,
		order.TaxAmount, order.TaxCGST, order.TaxSGST, order.TaxIGST, order.IsInterState,
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

// AllocateInvoiceNumber bumps org_invoice_sequences.last_seq atomically via
// UPSERT (DO UPDATE locks the row implicitly), then stamps the formatted
// number on the order. If the order already has an invoice_number, returns
// the existing one without consuming a sequence value (idempotent).
func (r *orderRepository) AllocateInvoiceNumber(ctx context.Context, orderID, orgID uuid.UUID, fyLabel string) (string, error) {
	// Short-circuit: if already allocated, return current value.
	var existing *string
	if err := r.db.QueryRow(ctx,
		`SELECT invoice_number FROM orders WHERE id = $1 AND org_id = $2`, orderID, orgID,
	).Scan(&existing); err != nil {
		return "", err
	}
	if existing != nil && *existing != "" {
		return *existing, nil
	}

	// Atomic increment-and-return for this org/FY pair.
	var seq int64
	err := r.db.QueryRow(ctx, `
		INSERT INTO org_invoice_sequences (org_id, fy_label, last_seq, updated_at)
		VALUES ($1, $2, 1, NOW())
		ON CONFLICT (org_id, fy_label) DO UPDATE
		SET last_seq = org_invoice_sequences.last_seq + 1, updated_at = NOW()
		RETURNING last_seq
	`, orgID, fyLabel).Scan(&seq)
	if err != nil {
		return "", err
	}

	number := fmt.Sprintf("INV/%s/%05d", fyLabel, seq)
	if _, err := r.db.Exec(ctx,
		`UPDATE orders SET invoice_number = $1, updated_at = NOW() WHERE id = $2 AND org_id = $3`,
		number, orderID, orgID,
	); err != nil {
		return "", err
	}
	return number, nil
}

// ListFiltered builds a dynamic WHERE clause from the filter struct and
// returns the matching orders plus the unpaginated total. Designed to back
// both UI list and CSV export.
func (r *orderRepository) ListFiltered(ctx context.Context, orgID uuid.UUID, f domain.OrderListFilters) ([]*domain.Order, int, error) {
	// Build WHERE incrementally.
	conds := []string{"org_id = $1"}
	args := []any{orgID}
	add := func(sql string, val any) {
		args = append(args, val)
		conds = append(conds, fmt.Sprintf(sql, len(args)))
	}
	if f.Status != "" {
		add("status = $%d", f.Status)
	}
	if f.PaymentStatus != "" {
		add("payment_status = $%d", f.PaymentStatus)
	}
	if f.OrderType != "" {
		add("order_type = $%d", f.OrderType)
	}
	if f.From != nil {
		add("created_at >= $%d", *f.From)
	}
	if f.To != nil {
		add("created_at <= $%d", *f.To)
	}
	if f.Search != "" {
		// Search by order id prefix OR user id prefix (cast UUIDs to text).
		args = append(args, f.Search+"%")
		conds = append(conds, fmt.Sprintf("(id::text ILIKE $%d OR user_id::text ILIKE $%d)", len(args), len(args)))
	}
	where := " WHERE " + strings.Join(conds, " AND ")

	// Count first (cheap with proper indexes — runs on the filtered set).
	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM orders`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Pagination.
	page := f.Page
	if page < 1 {
		page = 1
	}
	per := f.PerPage
	if per <= 0 {
		per = 25
	}
	if per > 200 {
		per = 200
	}
	offset := (page - 1) * per

	q := `SELECT ` + orderCols + ` FROM orders` + where + ` ORDER BY created_at DESC`
	// Per==0 (used by export path) skips LIMIT/OFFSET so all matches stream.
	if f.PerPage > 0 || f.Page > 0 {
		q += fmt.Sprintf(" LIMIT %d OFFSET %d", per, offset)
	}

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Order, 0)
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, o)
	}
	return items, total, rows.Err()
}

func (r *orderRepository) ListByCustomer(ctx context.Context, customerID uuid.UUID, cursorCreatedAt time.Time, cursorID uuid.UUID, limit int) ([]CustomerOrderRow, error) {
	args := []any{customerID}
	q := `
		SELECT id, org_id, customer_id, COALESCE(invoice_number,''), status, payment_status,
		       payment_id, COALESCE(subtotal,0), COALESCE(delivery_fee,0), COALESCE(discount,0),
		       total_amount, COALESCE(delivery_address_snapshot, '{}'::jsonb),
		       created_at, updated_at
		  FROM orders
		 WHERE customer_id = $1`
	if !cursorCreatedAt.IsZero() {
		args = append(args, cursorCreatedAt)
		args = append(args, cursorID)
		q += fmt.Sprintf(` AND (created_at < $%d OR (created_at = $%d AND id > $%d))`,
			len(args)-1, len(args)-1, len(args))
	}
	args = append(args, limit)
	q += ` ORDER BY created_at DESC, id ASC LIMIT $` + strconv.Itoa(len(args))

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil { return nil, err }
	defer rows.Close()
	out := []CustomerOrderRow{}
	for rows.Next() {
		var c CustomerOrderRow
		if err := rows.Scan(
			&c.ID, &c.OrgID, &c.CustomerID, &c.InvoiceNumber, &c.Status, &c.PaymentStatus,
			&c.PaymentID, &c.Subtotal, &c.DeliveryFee, &c.Discount,
			&c.TotalAmount, &c.DeliveryAddressSnapshot,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *orderRepository) GetByCustomerAndID(ctx context.Context, customerID, orderID uuid.UUID) (*CustomerOrderRow, []OrderItemRow, error) {
	var c CustomerOrderRow
	err := r.pool.QueryRow(ctx, `
		SELECT id, org_id, customer_id, COALESCE(invoice_number,''), status, payment_status,
		       payment_id, COALESCE(subtotal,0), COALESCE(delivery_fee,0), COALESCE(discount,0),
		       total_amount, COALESCE(delivery_address_snapshot, '{}'::jsonb),
		       created_at, updated_at
		  FROM orders
		 WHERE id = $1 AND customer_id = $2`,
		orderID, customerID,
	).Scan(
		&c.ID, &c.OrgID, &c.CustomerID, &c.InvoiceNumber, &c.Status, &c.PaymentStatus,
		&c.PaymentID, &c.Subtotal, &c.DeliveryFee, &c.Discount,
		&c.TotalAmount, &c.DeliveryAddressSnapshot,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, domain.ErrNotFound
	}
	if err != nil { return nil, nil, err }

	items, err := r.itemsForOrder(ctx, orderID)
	if err != nil { return nil, nil, err }
	return &c, items, nil
}

func (r *orderRepository) CancelByCustomer(tx pgx.Tx, customerID, orderID uuid.UUID, newStatus string) (*CancelSnapshot, error) {
	ctx := context.Background()
	var snap CancelSnapshot
	// Use a CTE to capture old_status before the update.
	err := tx.QueryRow(ctx, `
		WITH old AS (
			SELECT status, payment_status, payment_id, total_amount, org_id
			  FROM orders
			 WHERE id = $1
			   AND customer_id = $2
			   AND status IN ('pending', 'confirmed')
		)
		UPDATE orders
		   SET status = $3, cancelled_at = NOW(), updated_at = NOW()
		 WHERE id = $1
		   AND customer_id = $2
		   AND status IN ('pending', 'confirmed')
		   AND EXISTS (SELECT 1 FROM old)
		RETURNING
			(SELECT status FROM old),
			payment_status, payment_id, total_amount, org_id`,
		orderID, customerID, newStatus,
	).Scan(&snap.OldStatus, &snap.PaymentStatus, &snap.PaymentID, &snap.TotalAmount, &snap.OrgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil { return nil, err }

	rows, err := tx.Query(ctx, `
		SELECT oi.product_id, oi.quantity, oi.unit_price,
		       COALESCE(p.name, ''),
		       COALESCE(p.shop_slug, ''),
		       COALESCE(p.shop_image_urls[1], '')
		  FROM order_items oi
		  LEFT JOIN products p ON p.id = oi.product_id
		 WHERE oi.order_id = $1`,
		orderID,
	)
	if err != nil { return nil, err }
	defer rows.Close()
	for rows.Next() {
		var it OrderItemRow
		if err := rows.Scan(
			&it.ProductID, &it.Quantity, &it.UnitPrice,
			&it.ProductName, &it.ProductSlug, &it.ProductImage,
		); err != nil { return nil, err }
		snap.Items = append(snap.Items, it)
	}
	return &snap, rows.Err()
}

func (r *orderRepository) RestoreStock(ctx context.Context, tx pgx.Tx, items []OrderItemRow, orderID uuid.UUID) error {
	for _, it := range items {
		_, err := tx.Exec(ctx, `
			UPDATE inventory
			   SET quantity = quantity + $1, updated_at = NOW()
			 WHERE product_id = $2`,
			it.Quantity, it.ProductID,
		)
		if err != nil { return fmt.Errorf("restore stock product=%s: %w", it.ProductID, err) }
	}
	return nil
}

// itemsForOrder is shared by GetByCustomerAndID and CancelByCustomer (Task 5).
func (r *orderRepository) itemsForOrder(ctx context.Context, orderID uuid.UUID) ([]OrderItemRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT oi.product_id, oi.quantity, oi.unit_price,
		       p.name,
		       COALESCE(p.shop_slug,''),
		       COALESCE(p.shop_image_urls[1],'')
		  FROM order_items oi
		  LEFT JOIN products p ON p.id = oi.product_id
		 WHERE oi.order_id = $1
		 ORDER BY oi.created_at`,
		orderID,
	)
	if err != nil { return nil, err }
	defer rows.Close()
	out := []OrderItemRow{}
	for rows.Next() {
		var it OrderItemRow
		if err := rows.Scan(
			&it.ProductID, &it.Quantity, &it.UnitPrice,
			&it.ProductName, &it.ProductSlug, &it.ProductImage,
		); err != nil { return nil, err }
		out = append(out, it)
	}
	return out, rows.Err()
}
