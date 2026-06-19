package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type CouponRepository interface {
	Create(ctx context.Context, c *domain.Coupon) error
	Update(ctx context.Context, c *domain.Coupon) error
	GetByID(ctx context.Context, id, orgID uuid.UUID) (*domain.Coupon, error)
	GetByCode(ctx context.Context, orgID uuid.UUID, code string) (*domain.Coupon, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]*domain.Coupon, error)
	Delete(ctx context.Context, id, orgID uuid.UUID) error

	// IncrementUsage bumps usage_count by 1. Bounded by max_uses via the
	// service-layer check before this is called.
	IncrementUsage(ctx context.Context, id uuid.UUID) error

	// RecordOrderUse inserts the order_coupons join row with a snapshot of
	// the deduction.
	RecordOrderUse(ctx context.Context, oc *domain.OrderCoupon) error

	// GetForOrder returns the join row for an order, or nil/ErrNotFound.
	GetForOrder(ctx context.Context, orderID uuid.UUID) (*domain.OrderCoupon, error)
}

type couponRepository struct {
	db DBTX
}

func NewCouponRepository(db DBTX) CouponRepository {
	return &couponRepository{db: db}
}

const couponCols = `id, org_id, code, discount_type, discount_value, min_subtotal,
	max_uses, usage_count, expires_at, is_active, description, created_at, updated_at`

func (r *couponRepository) scan(row pgx.Row) (*domain.Coupon, error) {
	c := &domain.Coupon{}
	err := row.Scan(
		&c.ID, &c.OrgID, &c.Code, &c.DiscountType, &c.DiscountValue, &c.MinSubtotal,
		&c.MaxUses, &c.UsageCount, &c.ExpiresAt, &c.IsActive, &c.Description,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (r *couponRepository) Create(ctx context.Context, c *domain.Coupon) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	now := time.Now().UTC()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = now
	}
	c.UpdatedAt = now
	c.Code = strings.TrimSpace(c.Code)
	_, err := r.db.Exec(ctx, `
		INSERT INTO coupons (`+couponCols+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`,
		c.ID, c.OrgID, c.Code, c.DiscountType, c.DiscountValue, c.MinSubtotal,
		c.MaxUses, c.UsageCount, c.ExpiresAt, c.IsActive, c.Description,
		c.CreatedAt, c.UpdatedAt,
	)
	return err
}

func (r *couponRepository) Update(ctx context.Context, c *domain.Coupon) error {
	c.UpdatedAt = time.Now().UTC()
	res, err := r.db.Exec(ctx, `
		UPDATE coupons SET
		    code           = $1,
		    discount_type  = $2,
		    discount_value = $3,
		    min_subtotal   = $4,
		    max_uses       = $5,
		    expires_at     = $6,
		    is_active      = $7,
		    description    = $8,
		    updated_at     = $9
		WHERE id = $10 AND org_id = $11
	`,
		c.Code, c.DiscountType, c.DiscountValue, c.MinSubtotal,
		c.MaxUses, c.ExpiresAt, c.IsActive, c.Description, c.UpdatedAt,
		c.ID, c.OrgID,
	)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *couponRepository) GetByID(ctx context.Context, id, orgID uuid.UUID) (*domain.Coupon, error) {
	row := r.db.QueryRow(ctx, `SELECT `+couponCols+` FROM coupons WHERE id = $1 AND org_id = $2`, id, orgID)
	c, err := r.scan(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return c, nil
}

func (r *couponRepository) GetByCode(ctx context.Context, orgID uuid.UUID, code string) (*domain.Coupon, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+couponCols+` FROM coupons WHERE org_id = $1 AND LOWER(code) = LOWER($2)`,
		orgID, strings.TrimSpace(code),
	)
	c, err := r.scan(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return c, nil
}

func (r *couponRepository) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]*domain.Coupon, error) {
	rows, err := r.db.Query(ctx, `SELECT `+couponCols+` FROM coupons WHERE org_id = $1 ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*domain.Coupon, 0)
	for rows.Next() {
		c, err := r.scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *couponRepository) Delete(ctx context.Context, id, orgID uuid.UUID) error {
	res, err := r.db.Exec(ctx, `DELETE FROM coupons WHERE id = $1 AND org_id = $2`, id, orgID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *couponRepository) IncrementUsage(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE coupons SET usage_count = usage_count + 1, updated_at = NOW() WHERE id = $1`,
		id,
	)
	return err
}

func (r *couponRepository) RecordOrderUse(ctx context.Context, oc *domain.OrderCoupon) error {
	if oc.CreatedAt.IsZero() {
		oc.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO order_coupons (order_id, coupon_id, code_snapshot, amount_off, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (order_id) DO NOTHING
	`, oc.OrderID, oc.CouponID, oc.CodeSnapshot, oc.AmountOff, oc.CreatedAt)
	return err
}

func (r *couponRepository) GetForOrder(ctx context.Context, orderID uuid.UUID) (*domain.OrderCoupon, error) {
	row := r.db.QueryRow(ctx, `
		SELECT order_id, coupon_id, code_snapshot, amount_off, created_at
		FROM order_coupons WHERE order_id = $1
	`, orderID)
	oc := &domain.OrderCoupon{}
	err := row.Scan(&oc.OrderID, &oc.CouponID, &oc.CodeSnapshot, &oc.AmountOff, &oc.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return oc, nil
}
