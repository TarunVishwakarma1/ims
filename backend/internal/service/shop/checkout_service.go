package shop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// paymentCreator is the minimal interface from service.PaymentService that
// CheckoutService needs. Defined here to avoid an import cycle:
// shop → service → middleware → shop.
type paymentCreator interface {
	CreateOrder(ctx context.Context, orgID, orderID uuid.UUID, amount int64) (*domain.Payment, error)
}

// Sentinel errors returned by CheckoutService methods.
var (
	ErrCartEmpty            = errors.New("cart empty")
	ErrAddressRequired      = errors.New("address required")
	ErrInsufficientStock    = errors.New("insufficient stock")
	ErrInvalidPaymentMethod = errors.New("invalid payment method")
	ErrCODIneligible        = errors.New("cod ineligible")
	ErrShopClosed           = errors.New("shop closed")
)

// CheckoutService handles order placement (Summary + Place) for the B2C shop.
type CheckoutService interface {
	Summary(ctx context.Context, customerID, addressID uuid.UUID, couponCode string) (*CheckoutSummary, error)
	Place(ctx context.Context, in PlaceOrderInput) (*PlaceOrderResult, error)
	PaymentOptions(ctx context.Context, customerID uuid.UUID) ([]PaymentOption, error)
}

// couponValidator is the slice of CouponService that we need.
type couponValidator interface {
	Validate(ctx context.Context, orgID uuid.UUID, code string, subtotal int64) (*domain.Coupon, int64, error)
	Apply(ctx context.Context, c *domain.Coupon, orderID uuid.UUID, amountOff int64) error
}

// AppliedCoupon describes a coupon attached to a checkout summary or order.
type AppliedCoupon struct {
	Code         string `json:"code"`
	DiscountType string `json:"discount_type"`
	AmountOff    int64  `json:"amount_off_paise"`
}

// PaymentOption describes a single available payment method and whether it is
// currently enabled for the customer's cart.
type PaymentOption struct {
	ID       string `json:"id"`
	Enabled  bool   `json:"enabled"`
	MinPaise int64  `json:"min_paise,omitempty"`
	MaxPaise int64  `json:"max_paise,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// CheckoutSummary is the pre-checkout breakdown returned to the UI.
type CheckoutSummary struct {
	Items             []CartItemView `json:"items"`
	SubtotalPaise     int64          `json:"subtotal_paise"`
	GSTPaise          int64          `json:"gst_paise"`
	PlatformPaise     int64          `json:"platform_paise"`
	DiscountPaise     int64          `json:"discount_paise"`
	ShippingPaise     int64          `json:"shipping_paise"`
	FreeShipThreshold int64          `json:"free_ship_threshold_paise"`
	TotalPayablePaise int64          `json:"total_payable_paise"`
	Coupon            *AppliedCoupon `json:"coupon,omitempty"`
}

// PlaceOrderInput carries everything needed to place a B2C order.
type PlaceOrderInput struct {
	CustomerID    uuid.UUID
	AddressID     uuid.UUID
	PaymentMethod string // "razorpay" | "cod"
	CouponCode    string // optional, applied at place time after re-validation
	Notes         string
}

// PlaceOrderResult is returned after a successful order placement.
type PlaceOrderResult struct {
	OrderID         uuid.UUID `json:"order_id"`
	RazorpayOrderID string    `json:"razorpay_order_id,omitempty"`
	RazorpayKeyID   string    `json:"razorpay_key_id,omitempty"`
	PayablePaise    int64     `json:"payable_paise"`
	InvoiceNumber   string    `json:"invoice_number"`
}

type checkoutService struct {
	pool              *pgxpool.Pool
	orgID             uuid.UUID
	cartRepo          repository.CartRepository
	addrRepo          repository.CustomerAddressRepository
	paymentSvc        paymentCreator
	orderRepo         repository.OrderRepository
	couponSvc         couponValidator
	razorpayKeyID     string
	codMinPaise       int64
	codMaxPaise       int64
	platformPaise     int64
	shippingPaise     int64
	freeShipThreshold int64
}

// NewCheckoutService constructs a CheckoutService.
// paymentSvc may be nil when only testing the COD path.
// couponSvc may be nil when coupons are disabled.
func NewCheckoutService(
	pool *pgxpool.Pool,
	orgID uuid.UUID,
	cartRepo repository.CartRepository,
	addrRepo repository.CustomerAddressRepository,
	paymentSvc paymentCreator,
	orderRepo repository.OrderRepository,
	couponSvc couponValidator,
	razorpayKeyID string,
	codMinPaise int64,
	codMaxPaise int64,
	platformPaise int64,
	shippingPaise int64,
	freeShipThreshold int64,
) CheckoutService {
	return &checkoutService{
		pool:              pool,
		orgID:             orgID,
		cartRepo:          cartRepo,
		addrRepo:          addrRepo,
		paymentSvc:        paymentSvc,
		orderRepo:         orderRepo,
		couponSvc:         couponSvc,
		razorpayKeyID:     razorpayKeyID,
		codMinPaise:       codMinPaise,
		platformPaise:     platformPaise,
		codMaxPaise:       codMaxPaise,
		shippingPaise:     shippingPaise,
		freeShipThreshold: freeShipThreshold,
	}
}

// shippingFor returns the delivery fee for a given subtotal, applying the
// free-shipping threshold (0 threshold = always charge the flat fee).
func (s *checkoutService) shippingFor(subtotal int64) int64 {
	if s.freeShipThreshold > 0 && subtotal >= s.freeShipThreshold {
		return 0
	}
	return s.shippingPaise
}

// cartOrg returns the shop the cart is bound to (P4 phase 3) — the order, its
// items, coupon scope, payment and invoice are all created under THAT org. An
// unbound cart (shouldn't happen once it holds items) falls back to the default
// shop org so legacy single-shop carts keep working.
// shopClosed reports whether the shop is currently outside its business hours.
// Fails open: any lookup error (no profile, DB hiccup) returns false so a
// transient issue never blocks an order.
func (s *checkoutService) shopClosed(ctx context.Context, orgID uuid.UUID) bool {
	var opens, closes *string
	if err := s.pool.QueryRow(ctx,
		`SELECT to_char(opens_at, 'HH24:MI'), to_char(closes_at, 'HH24:MI')
		   FROM shop_profiles WHERE org_id = $1`, orgID,
	).Scan(&opens, &closes); err != nil {
		return false
	}
	return !ShopOpen(opens, closes, time.Now())
}

func (s *checkoutService) cartOrg(cart *domain.Cart) uuid.UUID {
	if cart.ShopOrgID != uuid.Nil {
		return cart.ShopOrgID
	}
	return s.orgID
}

// Summary returns a pre-checkout price breakdown for the customer's cart.
// If couponCode is non-empty and resolves to a valid coupon for the org +
// subtotal, the discount is applied and surfaced in the returned summary.
// An invalid coupon does not fail Summary — the caller can re-render and
// surface the failure to the user via the returned error.
func (s *checkoutService) Summary(ctx context.Context, customerID, addressID uuid.UUID, couponCode string) (*CheckoutSummary, error) {
	addr, err := s.addrRepo.GetByID(ctx, addressID)
	if err != nil {
		return nil, err
	}
	if addr == nil || addr.CustomerID != customerID {
		return nil, ErrAddressRequired
	}

	cart, err := s.cartRepo.Get(ctx, customerID)
	if err != nil {
		return nil, err
	}
	if len(cart.Items) == 0 {
		return nil, ErrCartEmpty
	}
	cartOrg := s.cartOrg(cart)

	var subtotal, gst int64
	views := make([]CartItemView, 0, len(cart.Items))

	for _, it := range cart.Items {
		subtotal += int64(it.Qty) * it.UnitPricePaise

		var (
			slug, name, image string
			rate, available   int
		)
		_ = s.pool.QueryRow(ctx, `
			SELECT COALESCE(p.shop_slug, ''),
			       p.name,
			       COALESCE(p.shop_image_urls[1], ''),
			       p.gst_rate,
			       COALESCE(i.quantity, 0)
			  FROM products p
			  LEFT JOIN inventory i ON i.product_id = p.id
			 WHERE p.id = $1
		`, it.ProductID).Scan(&slug, &name, &image, &rate, &available)

		gst += (int64(it.Qty) * it.UnitPricePaise * int64(rate)) / 100

		views = append(views, CartItemView{
			ProductID:      it.ProductID,
			Slug:           slug,
			Name:           name,
			Image:          image,
			Qty:            it.Qty,
			UnitPricePaise: it.UnitPricePaise,
			MaxQty:         available,
		})
	}

	platform := s.platformPaise

	var discount int64
	var applied *AppliedCoupon
	if couponCode != "" && s.couponSvc != nil {
		c, off, err := s.couponSvc.Validate(ctx, cartOrg, couponCode, subtotal)
		if err != nil {
			return nil, err
		}
		discount = off
		applied = &AppliedCoupon{
			Code:         c.Code,
			DiscountType: string(c.DiscountType),
			AmountOff:    off,
		}
	}

	shipping := s.shippingFor(subtotal)

	total := subtotal + gst + platform + shipping - discount
	if total < 0 {
		total = 0
	}

	return &CheckoutSummary{
		Items:             views,
		SubtotalPaise:     subtotal,
		GSTPaise:          gst,
		PlatformPaise:     platform,
		DiscountPaise:     discount,
		ShippingPaise:     shipping,
		FreeShipThreshold: s.freeShipThreshold,
		TotalPayablePaise: total,
		Coupon:            applied,
	}, nil
}

// Place validates inputs, atomically decrements inventory, creates the order
// and order_items inside a transaction, clears the cart, allocates an invoice
// number, and (for razorpay) calls paymentSvc.CreateOrder.
func (s *checkoutService) Place(ctx context.Context, in PlaceOrderInput) (*PlaceOrderResult, error) {
	// --- Validate inputs ---
	if in.AddressID == uuid.Nil {
		return nil, ErrAddressRequired
	}
	if in.PaymentMethod != "razorpay" && in.PaymentMethod != "cod" {
		return nil, ErrInvalidPaymentMethod
	}

	// --- Verify address ownership ---
	addr, err := s.addrRepo.GetByID(ctx, in.AddressID)
	if err != nil {
		return nil, err
	}
	if addr == nil || addr.CustomerID != in.CustomerID {
		return nil, ErrAddressRequired
	}

	// --- Load cart ---
	cart, err := s.cartRepo.Get(ctx, in.CustomerID)
	if err != nil {
		return nil, err
	}
	if len(cart.Items) == 0 {
		return nil, ErrCartEmpty
	}
	// The order is created under the cart's bound shop (P4 phase 3).
	cartOrg := s.cartOrg(cart)

	// --- Reject orders while the shop is outside its business hours ---
	if s.shopClosed(ctx, cartOrg) {
		return nil, ErrShopClosed
	}

	// --- Begin transaction ---
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// --- Atomically decrement stock and compute totals ---
	var subtotal, gst int64
	for _, it := range cart.Items {
		var remaining int64
		// Fix 4: inventory has UNIQUE on product_id only — no org_id filter.
		err := tx.QueryRow(ctx, `
			UPDATE inventory
			   SET quantity = quantity - $1
			 WHERE product_id = $2
			   AND quantity >= $1
			RETURNING quantity
		`, it.Qty, it.ProductID).Scan(&remaining)
		if err != nil {
			// No rows returned means stock was insufficient.
			return nil, ErrInsufficientStock
		}

		subtotal += int64(it.Qty) * it.UnitPricePaise

		var rate int
		_ = tx.QueryRow(ctx,
			`SELECT gst_rate FROM products WHERE id = $1`, it.ProductID,
		).Scan(&rate)
		gst += (int64(it.Qty) * it.UnitPricePaise * int64(rate)) / 100
	}
	platform := s.platformPaise

	// Re-validate the coupon inside the tx so usage caps remain consistent
	// across concurrent placements. Returns the discount we'll apply.
	var discount int64
	var couponDomain *domain.Coupon
	if in.CouponCode != "" && s.couponSvc != nil {
		c, off, err := s.couponSvc.Validate(ctx, cartOrg, in.CouponCode, subtotal)
		if err != nil {
			return nil, err
		}
		couponDomain = c
		discount = off
	}

	shipping := s.shippingFor(subtotal)

	total := subtotal + gst + platform + shipping - discount
	if total < 0 {
		total = 0
	}

	// --- COD totals round up to the nearest rupee so the rider doesn't have
	//     to carry paise change. Capture the adjustment as its own line so
	//     the invoice can show it explicitly. ---
	var codRound int64
	if in.PaymentMethod == "cod" {
		if rem := total % 100; rem != 0 {
			codRound = int64(100) - rem
			total += codRound
		}
		if total < s.codMinPaise || total > s.codMaxPaise {
			return nil, ErrCODIneligible
		}
	}

	// --- Determine order status ---
	orderStatus := "pending" // razorpay: wait for payment confirmation
	if in.PaymentMethod == "cod" {
		orderStatus = "confirmed"
	}

	// --- Insert order row ---
	orderID := uuid.New()
	addrID := in.AddressID
	custID := in.CustomerID

	// Snapshot the delivery address into the order so cancellations / edits /
	// deletions on the source address row cannot alter historical records.
	snapshot, err := json.Marshal(map[string]any{
		"name":    addr.Name,
		"phone":   addr.Phone,
		"line1":   addr.Line1,
		"line2":   addr.Line2,
		"city":    addr.City,
		"state":   addr.State,
		"pincode": addr.PostalCode,
	})
	if err != nil {
		return nil, fmt.Errorf("encode delivery snapshot: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO orders (
			id, org_id, customer_id, delivery_address_id,
			status, order_type, total_amount, subtotal,
			gst_paise, packing_paise, handling_paise, surge_paise,
			platform_paise, delivery_fee, discount, cod_round_paise,
			payment_status, payment_method, delivery_address_snapshot, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4,
			$5, 'b2c', $6, $7,
			$8, 0, 0, 0,
			$9, $10, $11, $12,
			'unpaid', $13, $14, NOW(), NOW()
		)
	`, orderID, cartOrg, custID, addrID, orderStatus, total, subtotal, gst, platform, shipping, discount, codRound, in.PaymentMethod, snapshot)
	if err != nil {
		return nil, fmt.Errorf("insert order: %w", err)
	}

	// Record coupon redemption + bump usage counter.
	if couponDomain != nil {
		if err := s.couponSvc.Apply(ctx, couponDomain, orderID, discount); err != nil {
			return nil, fmt.Errorf("apply coupon: %w", err)
		}
	}

	// Seed the timeline with a "placed" event (+ "confirmed" for COD which
	// skips the payment-pending hop).
	if _, err = tx.Exec(ctx,
		`INSERT INTO order_events (order_id, status, note) VALUES ($1, 'placed', '')`,
		orderID,
	); err != nil {
		return nil, fmt.Errorf("insert order_event placed: %w", err)
	}
	if orderStatus == "confirmed" {
		if _, err = tx.Exec(ctx,
			`INSERT INTO order_events (order_id, status, note) VALUES ($1, 'confirmed', 'COD — payment due at delivery')`,
			orderID,
		); err != nil {
			return nil, fmt.Errorf("insert order_event confirmed: %w", err)
		}
	}

	// --- Insert order_items ---
	for _, it := range cart.Items {
		itemID := uuid.New()
		_, err = tx.Exec(ctx, `
			INSERT INTO order_items (id, org_id, order_id, product_id, quantity, unit_price)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, itemID, cartOrg, orderID, it.ProductID, it.Qty, it.UnitPricePaise)
		if err != nil {
			return nil, fmt.Errorf("insert order_item: %w", err)
		}
	}

	// --- Clear cart ---
	_, err = tx.Exec(ctx,
		`DELETE FROM customer_cart_items WHERE customer_id = $1`, in.CustomerID,
	)
	if err != nil {
		return nil, fmt.Errorf("clear cart: %w", err)
	}

	// --- Commit ---
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// --- Allocate invoice number (after commit — AllocateInvoiceNumber uses pool) ---
	fy := indianFYLabel(time.Now().UTC())
	invNum, err := s.orderRepo.AllocateInvoiceNumber(ctx, orderID, cartOrg, fy)
	if err != nil {
		zap.L().Error("allocate invoice number failed",
			zap.String("order_id", orderID.String()),
			zap.Error(err))
		invNum = ""
	}

	res := &PlaceOrderResult{
		OrderID:       orderID,
		PayablePaise:  total,
		InvoiceNumber: invNum,
	}

	// --- Payment dispatch (Razorpay only) ---
	if in.PaymentMethod == "razorpay" {
		if s.paymentSvc == nil {
			return nil, fmt.Errorf("payment service not configured")
		}
		payment, err := s.paymentSvc.CreateOrder(ctx, cartOrg, orderID, total)
		if err != nil {
			return nil, fmt.Errorf("razorpay create: %w", err)
		}
		if payment != nil {
			res.RazorpayOrderID = payment.RazorpayOrderID
			res.RazorpayKeyID = s.razorpayKeyID
		}
	}

	return res, nil
}

// PaymentOptions evaluates the customer's cart total against COD bounds and
// returns available payment methods. An empty cart returns COD enabled (no gate).
func (s *checkoutService) PaymentOptions(ctx context.Context, customerID uuid.UUID) ([]PaymentOption, error) {
	cart, err := s.cartRepo.Get(ctx, customerID)
	if err != nil {
		return nil, err
	}
	var subtotal, gst int64
	for _, it := range cart.Items {
		subtotal += int64(it.Qty) * it.UnitPricePaise
		var rate int
		_ = s.pool.QueryRow(ctx, `SELECT gst_rate FROM products WHERE id=$1`, it.ProductID).Scan(&rate)
		gst += (int64(it.Qty) * it.UnitPricePaise * int64(rate)) / 100
	}
	total := subtotal + gst

	cod := PaymentOption{ID: "cod", Enabled: true, MinPaise: s.codMinPaise, MaxPaise: s.codMaxPaise}
	if total > 0 && total < s.codMinPaise {
		cod.Enabled = false
		cod.Reason = "min_value_below"
	} else if total > s.codMaxPaise {
		cod.Enabled = false
		cod.Reason = "max_value_exceeded"
	}

	return []PaymentOption{
		{ID: "razorpay", Enabled: true},
		cod,
	}, nil
}

// indianFYLabel returns the Indian financial year label for the given time.
// For example, for April 2025 – March 2026, it returns "2025-26".
func indianFYLabel(t time.Time) string {
	y := t.Year()
	if t.Month() < 4 {
		y--
	}
	return fmt.Sprintf("%d-%02d", y, (y+1)%100)
}
