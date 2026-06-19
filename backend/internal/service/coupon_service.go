package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/google/uuid"
)

// CouponService owns coupon CRUD plus the discount calculation used at
// checkout. Validation walks the same checks the API exposes so callers
// pre-flight without committing the order.
type CouponService interface {
	Create(ctx context.Context, c *domain.Coupon, orgID uuid.UUID) error
	Update(ctx context.Context, c *domain.Coupon, orgID uuid.UUID) error
	Delete(ctx context.Context, id, orgID uuid.UUID) error
	List(ctx context.Context, orgID uuid.UUID) ([]*domain.Coupon, error)
	GetByID(ctx context.Context, id, orgID uuid.UUID) (*domain.Coupon, error)

	// Validate checks the coupon against the provided subtotal (paise) and
	// supplier_org_id. Returns the discount amount (paise) and the coupon
	// row, or an error explaining the rejection. Does NOT increment usage.
	Validate(ctx context.Context, supplierOrgID uuid.UUID, code string, subtotal int64) (*domain.Coupon, int64, error)

	// Apply records the coupon → order use and bumps the coupon's usage
	// count atomically. Idempotent on order_id (ON CONFLICT DO NOTHING).
	Apply(ctx context.Context, c *domain.Coupon, orderID uuid.UUID, amountOff int64) error
}

type couponService struct {
	repo repository.CouponRepository
}

func NewCouponService(repo repository.CouponRepository) CouponService {
	return &couponService{repo: repo}
}

func (s *couponService) Create(ctx context.Context, c *domain.Coupon, orgID uuid.UUID) error {
	c.OrgID = orgID
	c.Code = strings.TrimSpace(c.Code)
	if c.Code == "" {
		return errors.New("code is required")
	}
	if c.DiscountType != domain.CouponTypePercent && c.DiscountType != domain.CouponTypeFixed {
		return errors.New("discount_type must be 'percent' or 'fixed'")
	}
	if c.DiscountValue <= 0 {
		return errors.New("discount_value must be positive")
	}
	if c.DiscountType == domain.CouponTypePercent && c.DiscountValue > 100 {
		return errors.New("percent discount cannot exceed 100")
	}
	return s.repo.Create(ctx, c)
}

func (s *couponService) Update(ctx context.Context, c *domain.Coupon, orgID uuid.UUID) error {
	existing, err := s.repo.GetByID(ctx, c.ID, orgID)
	if err != nil {
		return err
	}
	// Preserve immutable bookkeeping fields. Edit treats the request as a
	// patch — usage_count and created_at must never come from the client.
	c.OrgID = existing.OrgID
	c.UsageCount = existing.UsageCount
	c.CreatedAt = existing.CreatedAt
	if c.DiscountType == domain.CouponTypePercent && c.DiscountValue > 100 {
		return errors.New("percent discount cannot exceed 100")
	}
	return s.repo.Update(ctx, c)
}

func (s *couponService) Delete(ctx context.Context, id, orgID uuid.UUID) error {
	return s.repo.Delete(ctx, id, orgID)
}

func (s *couponService) List(ctx context.Context, orgID uuid.UUID) ([]*domain.Coupon, error) {
	return s.repo.ListByOrg(ctx, orgID)
}

func (s *couponService) GetByID(ctx context.Context, id, orgID uuid.UUID) (*domain.Coupon, error) {
	return s.repo.GetByID(ctx, id, orgID)
}

func (s *couponService) Validate(ctx context.Context, supplierOrgID uuid.UUID, code string, subtotal int64) (*domain.Coupon, int64, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, 0, errors.New("coupon code is required")
	}
	c, err := s.repo.GetByCode(ctx, supplierOrgID, code)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, 0, errors.New("invalid coupon code")
		}
		return nil, 0, err
	}
	if !c.IsActive {
		return nil, 0, errors.New("coupon is no longer active")
	}
	if c.ExpiresAt != nil && time.Now().UTC().After(*c.ExpiresAt) {
		return nil, 0, errors.New("coupon has expired")
	}
	if c.MaxUses != nil && c.UsageCount >= *c.MaxUses {
		return nil, 0, errors.New("coupon usage limit reached")
	}
	if subtotal < c.MinSubtotal {
		return nil, 0, fmt.Errorf("minimum order subtotal for this coupon is %.2f", float64(c.MinSubtotal)/100.0)
	}

	amountOff := calcDiscount(c, subtotal)
	if amountOff <= 0 {
		return nil, 0, errors.New("coupon yields no discount on this order")
	}
	return c, amountOff, nil
}

func (s *couponService) Apply(ctx context.Context, c *domain.Coupon, orderID uuid.UUID, amountOff int64) error {
	if err := s.repo.RecordOrderUse(ctx, &domain.OrderCoupon{
		OrderID:      orderID,
		CouponID:     c.ID,
		CodeSnapshot: c.Code,
		AmountOff:    amountOff,
	}); err != nil {
		return err
	}
	return s.repo.IncrementUsage(ctx, c.ID)
}

// calcDiscount caps the discount at the subtotal — never produce a negative
// total. For percent: floor(subtotal * value / 100). For fixed: value.
func calcDiscount(c *domain.Coupon, subtotal int64) int64 {
	var off int64
	switch c.DiscountType {
	case domain.CouponTypePercent:
		off = subtotal * c.DiscountValue / 100
	case domain.CouponTypeFixed:
		off = c.DiscountValue
	}
	if off > subtotal {
		off = subtotal
	}
	return off
}
