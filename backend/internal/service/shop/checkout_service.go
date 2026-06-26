package shop

import (
	"context"
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
)

// CheckoutService handles order placement (Summary + Place) for the B2C shop.
type CheckoutService interface {
	Summary(ctx context.Context, customerID, addressID uuid.UUID) (*CheckoutSummary, error)
	Place(ctx context.Context, in PlaceOrderInput) (*PlaceOrderResult, error)
	PaymentOptions(ctx context.Context, customerID uuid.UUID) ([]PaymentOption, error)
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
	ShippingPaise     int64          `json:"shipping_paise"`
	TotalPayablePaise int64          `json:"total_payable_paise"`
}

// PlaceOrderInput carries everything needed to place a B2C order.
type PlaceOrderInput struct {
	CustomerID    uuid.UUID
	AddressID     uuid.UUID
	PaymentMethod string // "razorpay" | "cod"
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
	pool          *pgxpool.Pool
	orgID         uuid.UUID
	cartRepo      repository.CartRepository
	addrRepo      repository.CustomerAddressRepository
	paymentSvc    paymentCreator
	orderRepo     repository.OrderRepository
	razorpayKeyID string
	codMinPaise   int64
	codMaxPaise   int64
}

// NewCheckoutService constructs a CheckoutService.
// paymentSvc may be nil when only testing the COD path.
func NewCheckoutService(
	pool *pgxpool.Pool,
	orgID uuid.UUID,
	cartRepo repository.CartRepository,
	addrRepo repository.CustomerAddressRepository,
	paymentSvc paymentCreator,
	orderRepo repository.OrderRepository,
	razorpayKeyID string,
	codMinPaise int64,
	codMaxPaise int64,
) CheckoutService {
	return &checkoutService{
		pool:          pool,
		orgID:         orgID,
		cartRepo:      cartRepo,
		addrRepo:      addrRepo,
		paymentSvc:    paymentSvc,
		orderRepo:     orderRepo,
		razorpayKeyID: razorpayKeyID,
		codMinPaise:   codMinPaise,
		codMaxPaise:   codMaxPaise,
	}
}

// Summary returns a pre-checkout price breakdown for the customer's cart.
// V1: ShippingPaise = 0 (free shipping).
func (s *checkoutService) Summary(ctx context.Context, customerID, addressID uuid.UUID) (*CheckoutSummary, error) {
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

	var subtotal, gst int64
	views := make([]CartItemView, 0, len(cart.Items))

	for _, it := range cart.Items {
		subtotal += int64(it.Qty) * it.UnitPricePaise

		var (
			slug, name, image string
			rate, available   int
		)
		_ = s.pool.QueryRow(ctx, `
			SELECT p.slug,
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

	return &CheckoutSummary{
		Items:             views,
		SubtotalPaise:     subtotal,
		GSTPaise:          gst,
		ShippingPaise:     0,
		TotalPayablePaise: subtotal + gst,
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
	total := subtotal + gst

	// --- Gate COD by min/max bounds ---
	if in.PaymentMethod == "cod" {
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
	_, err = tx.Exec(ctx, `
		INSERT INTO orders (
			id, org_id, customer_id, delivery_address_id,
			status, order_type, total_amount, subtotal,
			payment_status, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4,
			$5, 'b2c', $6, $7,
			'unpaid', NOW(), NOW()
		)
	`, orderID, s.orgID, custID, addrID, orderStatus, total, subtotal)
	if err != nil {
		return nil, fmt.Errorf("insert order: %w", err)
	}

	// --- Insert order_items ---
	for _, it := range cart.Items {
		itemID := uuid.New()
		_, err = tx.Exec(ctx, `
			INSERT INTO order_items (id, org_id, order_id, product_id, quantity, unit_price)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, itemID, s.orgID, orderID, it.ProductID, it.Qty, it.UnitPricePaise)
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
	invNum, err := s.orderRepo.AllocateInvoiceNumber(ctx, orderID, s.orgID, fy)
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
		payment, err := s.paymentSvc.CreateOrder(ctx, s.orgID, orderID, total)
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
