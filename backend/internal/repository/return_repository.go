package repository

import (
	"context"
	"errors"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ReturnRepository interface {
	Create(ctx context.Context, req *domain.ReturnRequest) error
	CreateItem(ctx context.Context, item *domain.ReturnItem) error
	GetByID(ctx context.Context, id, orgID uuid.UUID) (*domain.ReturnRequest, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]*domain.ReturnRequest, error)
	ListByOrder(ctx context.Context, orderID, orgID uuid.UUID) ([]*domain.ReturnRequest, error)
	ListItems(ctx context.Context, returnID uuid.UUID) ([]*domain.ReturnItem, error)
	// PriorReturnedByItem returns a map of order_item_id → already-returned
	// quantity, summed across every NON-rejected prior return for the order.
	// Drives partial-return eligibility ("can return at most ordered minus
	// what's already been returned").
	PriorReturnedByItem(ctx context.Context, orderID uuid.UUID) (map[uuid.UUID]int, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.ReturnStatus, note *string) error
	WithTx(tx pgx.Tx) ReturnRepository
	BeginTx(ctx context.Context) (pgx.Tx, error)
}

type returnRepository struct {
	db   DBTX
	pool *pgxpool.Pool
}

func NewReturnRepository(pool *pgxpool.Pool) ReturnRepository {
	return &returnRepository{db: pool, pool: pool}
}

func (r *returnRepository) WithTx(tx pgx.Tx) ReturnRepository {
	return &returnRepository{db: tx, pool: r.pool}
}

func (r *returnRepository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	if r.pool == nil {
		return nil, errors.New("no pool available for transaction")
	}
	return r.pool.Begin(ctx)
}

const returnCols = `id, org_id, order_id, requested_by, status, reason, refund_amount,
		rejection_note, created_at, updated_at, approved_at, received_at, refunded_at`

func scanReturn(scanner interface {
	Scan(dest ...any) error
}) (*domain.ReturnRequest, error) {
	rr := &domain.ReturnRequest{}
	err := scanner.Scan(
		&rr.ID, &rr.OrgID, &rr.OrderID, &rr.RequestedBy, &rr.Status, &rr.Reason, &rr.RefundAmount,
		&rr.RejectionNote, &rr.CreatedAt, &rr.UpdatedAt, &rr.ApprovedAt, &rr.ReceivedAt, &rr.RefundedAt,
	)
	if err != nil {
		return nil, err
	}
	return rr, nil
}

func (r *returnRepository) Create(ctx context.Context, req *domain.ReturnRequest) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO return_requests (id, org_id, order_id, requested_by, status, reason, refund_amount, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, req.ID, req.OrgID, req.OrderID, req.RequestedBy, req.Status, req.Reason, req.RefundAmount, req.CreatedAt, req.UpdatedAt)
	return err
}

func (r *returnRepository) CreateItem(ctx context.Context, item *domain.ReturnItem) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO return_items (id, return_id, order_item_id, product_id, quantity, unit_price)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, item.ID, item.ReturnID, item.OrderItemID, item.ProductID, item.Quantity, item.UnitPrice)
	return err
}

func (r *returnRepository) GetByID(ctx context.Context, id, orgID uuid.UUID) (*domain.ReturnRequest, error) {
	// Allow access if either side of the trade matches the caller's org:
	// buyer (rr.org_id) or supplier (orders.supplier_org_id on the linked
	// order). Direction tag tells the UI which role to surface.
	row := r.db.QueryRow(ctx, `
		SELECT rr.id, rr.org_id, rr.order_id, rr.requested_by, rr.status, rr.reason, rr.refund_amount,
		       rr.rejection_note, rr.created_at, rr.updated_at, rr.approved_at, rr.received_at, rr.refunded_at,
		       CASE
		         WHEN rr.org_id = $2 THEN 'outgoing'
		         WHEN o.supplier_org_id = $2 THEN 'incoming'
		         ELSE ''
		       END AS direction
		FROM return_requests rr
		JOIN orders o ON o.id = rr.order_id
		WHERE rr.id = $1 AND (rr.org_id = $2 OR o.supplier_org_id = $2)
	`, id, orgID)
	rr := &domain.ReturnRequest{}
	err := row.Scan(
		&rr.ID, &rr.OrgID, &rr.OrderID, &rr.RequestedBy, &rr.Status, &rr.Reason, &rr.RefundAmount,
		&rr.RejectionNote, &rr.CreatedAt, &rr.UpdatedAt, &rr.ApprovedAt, &rr.ReceivedAt, &rr.RefundedAt,
		&rr.Direction,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	items, err := r.ListItems(ctx, rr.ID)
	if err != nil {
		return nil, err
	}
	rr.Items = items
	return rr, nil
}

// ListByOrg returns every return request the caller's org is involved in,
// in either direction. JOIN on orders adds the direction tag without an
// extra round-trip:
//   - direction = "outgoing" when return.org_id == caller (we're the buyer)
//   - direction = "incoming" when orders.supplier_org_id == caller (we're
//     the supplier receiving a return)
func (r *returnRepository) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]*domain.ReturnRequest, error) {
	rows, err := r.db.Query(ctx, `
		SELECT rr.id, rr.org_id, rr.order_id, rr.requested_by, rr.status, rr.reason, rr.refund_amount,
		       rr.rejection_note, rr.created_at, rr.updated_at, rr.approved_at, rr.received_at, rr.refunded_at,
		       CASE
		         WHEN rr.org_id = $1 THEN 'outgoing'
		         WHEN o.supplier_org_id = $1 THEN 'incoming'
		         ELSE ''
		       END AS direction
		FROM return_requests rr
		JOIN orders o ON o.id = rr.order_id
		WHERE rr.org_id = $1 OR o.supplier_org_id = $1
		ORDER BY rr.created_at DESC
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*domain.ReturnRequest, 0)
	for rows.Next() {
		rr := &domain.ReturnRequest{}
		if err := rows.Scan(
			&rr.ID, &rr.OrgID, &rr.OrderID, &rr.RequestedBy, &rr.Status, &rr.Reason, &rr.RefundAmount,
			&rr.RejectionNote, &rr.CreatedAt, &rr.UpdatedAt, &rr.ApprovedAt, &rr.ReceivedAt, &rr.RefundedAt,
			&rr.Direction,
		); err != nil {
			return nil, err
		}
		out = append(out, rr)
	}
	return out, rows.Err()
}

func (r *returnRepository) ListByOrder(ctx context.Context, orderID, orgID uuid.UUID) ([]*domain.ReturnRequest, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+returnCols+` FROM return_requests WHERE order_id = $1 AND org_id = $2 ORDER BY created_at DESC`,
		orderID, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*domain.ReturnRequest, 0)
	for rows.Next() {
		rr, err := scanReturn(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rr)
	}
	return out, rows.Err()
}

func (r *returnRepository) ListItems(ctx context.Context, returnID uuid.UUID) ([]*domain.ReturnItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, return_id, order_item_id, product_id, quantity, unit_price
		FROM return_items WHERE return_id = $1
	`, returnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*domain.ReturnItem, 0)
	for rows.Next() {
		it := &domain.ReturnItem{}
		if err := rows.Scan(&it.ID, &it.ReturnID, &it.OrderItemID, &it.ProductID, &it.Quantity, &it.UnitPrice); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (r *returnRepository) PriorReturnedByItem(ctx context.Context, orderID uuid.UUID) (map[uuid.UUID]int, error) {
	// Exclude rejected returns — rejected items never left the customer's
	// hands, so they remain returnable. requested/approved/in_transit are
	// included as "in-flight" returns: pessimistic but prevents double-spending.
	rows, err := r.db.Query(ctx, `
		SELECT ri.order_item_id, COALESCE(SUM(ri.quantity), 0)
		FROM return_items ri
		JOIN return_requests rr ON rr.id = ri.return_id
		WHERE rr.order_id = $1 AND rr.status != 'rejected'
		GROUP BY ri.order_item_id
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[uuid.UUID]int)
	for rows.Next() {
		var k uuid.UUID
		var v int
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

// UpdateStatus flips status + the corresponding timestamp column. note is
// only persisted on rejected transitions; ignored for others.
func (r *returnRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.ReturnStatus, note *string) error {
	// Single UPDATE that conditionally sets the matching timestamp.
	_, err := r.db.Exec(ctx, `
		UPDATE return_requests SET
			status = $2,
			rejection_note = COALESCE($3, rejection_note),
			approved_at  = CASE WHEN $2 = 'approved'  THEN NOW() ELSE approved_at  END,
			received_at  = CASE WHEN $2 = 'received'  THEN NOW() ELSE received_at  END,
			refunded_at  = CASE WHEN $2 = 'refunded'  THEN NOW() ELSE refunded_at  END,
			updated_at   = NOW()
		WHERE id = $1
	`, id, status, note)
	return err
}
