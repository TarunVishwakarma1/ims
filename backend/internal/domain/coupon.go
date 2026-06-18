package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	CouponTypePercent = "percent"
	CouponTypeFixed   = "fixed"
)

// Coupon is a supplier-scoped discount code. percent stores e.g. 10 for 10%;
// fixed stores paise.
type Coupon struct {
	ID            uuid.UUID  `json:"id"`
	OrgID         uuid.UUID  `json:"org_id"`
	Code          string     `json:"code"`
	DiscountType  string     `json:"discount_type"` // percent | fixed
	DiscountValue int64      `json:"discount_value"`
	MinSubtotal   int64      `json:"min_subtotal"`
	MaxUses       *int       `json:"max_uses"`
	UsageCount    int        `json:"usage_count"`
	ExpiresAt     *time.Time `json:"expires_at"`
	IsActive      bool       `json:"is_active"`
	Description   *string    `json:"description"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// OrderCoupon is the join row: which coupon paid which order, and the
// snapshot of the deduction so future coupon edits don't rewrite history.
type OrderCoupon struct {
	OrderID      uuid.UUID `json:"order_id"`
	CouponID     uuid.UUID `json:"coupon_id"`
	CodeSnapshot string    `json:"code_snapshot"`
	AmountOff    int64     `json:"amount_off"`
	CreatedAt    time.Time `json:"created_at"`
}
