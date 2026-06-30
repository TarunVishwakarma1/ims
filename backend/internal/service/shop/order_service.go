package shop

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
)

type OrderListQuery struct {
	Limit  int    `json:"limit,omitempty"`
	Cursor string `json:"cursor,omitempty"`
}

type OrderListResult struct {
	Items      []OrderCard `json:"items"`
	NextCursor string      `json:"next_cursor,omitempty"`
}

type OrderCard struct {
	ID             uuid.UUID `json:"id"`
	InvoiceNumber  string    `json:"invoice_number"`
	Status         string    `json:"status"`
	PaymentStatus  string    `json:"payment_status"`
	PaymentMethod  string    `json:"payment_method,omitempty"`
	TotalAmount    int64     `json:"total_paise"`
	CreatedAt      time.Time `json:"created_at"`
	ItemCount      int       `json:"item_count"`
	FirstItemName  string    `json:"first_item_name"`
	FirstItemImage string    `json:"first_item_image,omitempty"`
}

type OrderDetail struct {
	OrderCard
	Subtotal    int64           `json:"subtotal_paise"`
	DeliveryFee int64           `json:"delivery_fee_paise"`
	Discount    int64           `json:"discount_paise"`
	Charges     []ChargeLine    `json:"charges"`
	Items       []OrderItemView `json:"items"`
	Address     map[string]any  `json:"delivery_address"`
	Timeline    []OrderEvent    `json:"timeline"`
	Cancellable bool            `json:"cancellable"`
}

// ChargeLine is a single row in the invoice price breakdown.
// When Struck=true the line is waived for this customer — the UI renders
// OriginalPaise with a strikethrough plus a "Free" label so the customer
// sees what they would otherwise have paid.
type ChargeLine struct {
	Label         string `json:"label"`
	Paise         int64  `json:"paise"`
	OriginalPaise int64  `json:"original_paise,omitempty"`
	Struck        bool   `json:"struck"`
}

// Default "would have cost" amounts for charges the shop currently waives.
// Persisted on the order row in a later iteration; today these are display
// hints sourced from constants.
const (
	defaultPackingPaise  = 500  // ₹5
	defaultHandlingPaise = 1000 // ₹10
	defaultShippingPaise = 4000 // ₹40
	defaultSurgePaise    = 1500 // ₹15
	defaultPlatformPaise = 300  // ₹3
)

type OrderItemView struct {
	ProductID uuid.UUID `json:"product_id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	Image     string    `json:"image"`
	Qty       int       `json:"qty"`
	UnitPrice int64     `json:"unit_price_paise"`
}

type OrderEvent struct {
	At     time.Time `json:"at"`
	Status string    `json:"status"`
	Note   string    `json:"note,omitempty"`
}

type CancelResult struct {
	Status        string `json:"status"`
	RefundQueued  bool   `json:"refund_queued"`
	EstimatedDays int    `json:"estimated_days,omitempty"`
}

// AdminOrderCard is a row in the admin shop-order list.
type AdminOrderCard struct {
	ID            uuid.UUID `json:"id"`
	InvoiceNumber string    `json:"invoice_number"`
	CustomerName  string    `json:"customer_name"`
	CustomerPhone string    `json:"customer_phone"`
	Status        string    `json:"status"`
	PaymentStatus string    `json:"payment_status"`
	PaymentMethod string    `json:"payment_method,omitempty"`
	TotalAmount   int64     `json:"total_paise"`
	ItemCount     int       `json:"item_count"`
	CreatedAt     time.Time `json:"created_at"`
}

type ShopOrderService interface {
	List(ctx context.Context, customerID uuid.UUID, q OrderListQuery) (*OrderListResult, error)
	Get(ctx context.Context, customerID, orderID uuid.UUID) (*OrderDetail, error)
	Cancel(ctx context.Context, customerID, orderID uuid.UUID) (*CancelResult, error)

	// Admin (org-scoped, not customer-scoped).
	AdminList(ctx context.Context, status string, limit, offset int) ([]AdminOrderCard, error)
	// AdvanceStatus moves a b2c order to nextStatus (state machine enforced).
	// Returns the owning customer id so the caller can notify them.
	AdvanceStatus(ctx context.Context, orderID uuid.UUID, nextStatus string) (uuid.UUID, error)
}

var (
	ErrOrderNotFound     = errors.New("order not found")
	ErrCancelNotAllowed  = errors.New("order status does not allow cancel")
	ErrRefundFailed      = errors.New("refund request failed")
	ErrInvalidCursor     = errors.New("invalid cursor")
	ErrInvalidTransition = errors.New("status transition not allowed")
)

// shopStatusTransitions is the b2c order state machine for admin actions.
// (Customer-initiated cancel is handled separately by Cancel.)
var shopStatusTransitions = map[string][]string{
	"pending":   {"confirmed", "cancelled"},
	"confirmed": {"shipped", "cancelled"},
	"shipped":   {"delivered"},
}

func canAdvance(from, to string) bool {
	for _, s := range shopStatusTransitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// paymentRefunder is the subset of service.PaymentService we need.
// Defined here to avoid importing the parent service package and creating
// an import cycle.
type paymentRefunder interface {
	Refund(ctx context.Context, orgID, paymentID uuid.UUID, amount int64, reason string) error
}

type shopOrderService struct {
	pool      *pgxpool.Pool
	repo      repository.OrderRepository
	refunder  paymentRefunder
	shopOrgID uuid.UUID
}

func NewShopOrderService(pool *pgxpool.Pool, repo repository.OrderRepository, refunder paymentRefunder, shopOrgID uuid.UUID) ShopOrderService {
	return &shopOrderService{pool, repo, refunder, shopOrgID}
}

type orderCursorPayload struct {
	K time.Time `json:"k"`
	I uuid.UUID `json:"i"`
}

func encodeOrderCursor(at time.Time, id uuid.UUID) string {
	b, _ := json.Marshal(orderCursorPayload{K: at, I: id})
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeOrderCursor(s string) (time.Time, uuid.UUID, error) {
	if s == "" {
		return time.Time{}, uuid.Nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidCursor
	}
	var p orderCursorPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidCursor
	}
	return p.K, p.I, nil
}

func (s *shopOrderService) List(ctx context.Context, customerID uuid.UUID, q OrderListQuery) (*OrderListResult, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	cursorAt, cursorID, err := decodeOrderCursor(q.Cursor)
	if err != nil {
		return nil, err
	}

	rows, err := s.repo.ListByCustomer(ctx, customerID, cursorAt, cursorID, limit+1)
	if err != nil {
		return nil, err
	}

	out := &OrderListResult{Items: []OrderCard{}}
	more := false
	if len(rows) > limit {
		rows = rows[:limit]
		more = true
	}

	for _, r := range rows {
		card := OrderCard{
			ID:            r.ID,
			InvoiceNumber: r.InvoiceNumber,
			Status:        r.Status,
			PaymentStatus: r.PaymentStatus,
			PaymentMethod: r.PaymentMethod,
			TotalAmount:   r.TotalAmount,
			CreatedAt:     r.CreatedAt,
		}
		// Pull item count + first item via separate query.
		// V1 keeps it simple: one query per order. Optimize in follow-up.
		items, err := s.firstItemAndCount(ctx, r.ID)
		if err != nil {
			return nil, err
		}
		card.ItemCount = items.count
		card.FirstItemName = items.firstName
		card.FirstItemImage = items.firstImage
		out.Items = append(out.Items, card)
	}

	if more && len(out.Items) > 0 {
		last := rows[len(rows)-1]
		out.NextCursor = encodeOrderCursor(last.CreatedAt, last.ID)
	}
	return out, nil
}

type orderItemsSummary struct {
	count      int
	firstName  string
	firstImage string
}

func (s *shopOrderService) firstItemAndCount(ctx context.Context, orderID uuid.UUID) (orderItemsSummary, error) {
	var summary orderItemsSummary
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FILTER (WHERE oi.id IS NOT NULL),
		       COALESCE(MAX(p.name) FILTER (WHERE rn = 1), ''),
		       COALESCE(MAX(COALESCE(p.shop_image_urls[1],'')) FILTER (WHERE rn = 1), '')
		  FROM (
		    SELECT oi.id, oi.product_id,
		           ROW_NUMBER() OVER (ORDER BY oi.created_at) AS rn
		      FROM order_items oi
		     WHERE oi.order_id = $1
		  ) oi
		  LEFT JOIN products p ON p.id = oi.product_id`,
		orderID,
	).Scan(&summary.count, &summary.firstName, &summary.firstImage)
	return summary, err
}

func (s *shopOrderService) Get(ctx context.Context, customerID, orderID uuid.UUID) (*OrderDetail, error) {
	row, items, err := s.repo.GetByCustomerAndID(ctx, customerID, orderID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}

	var addr map[string]any
	if len(row.DeliveryAddressSnapshot) > 0 {
		_ = json.Unmarshal(row.DeliveryAddressSnapshot, &addr)
	}

	views := make([]OrderItemView, 0, len(items))
	var firstName, firstImage string
	for i, it := range items {
		v := OrderItemView{
			ProductID: it.ProductID,
			Slug:      it.ProductSlug,
			Name:      it.ProductName,
			Image:     it.ProductImage,
			Qty:       it.Quantity,
			UnitPrice: it.UnitPrice,
		}
		views = append(views, v)
		if i == 0 {
			firstName = it.ProductName
			firstImage = it.ProductImage
		}
	}

	cancellable := row.Status == "pending" || row.Status == "confirmed"

	return &OrderDetail{
		OrderCard: OrderCard{
			ID:             row.ID,
			InvoiceNumber:  row.InvoiceNumber,
			Status:         row.Status,
			PaymentStatus:  row.PaymentStatus,
			PaymentMethod:  row.PaymentMethod,
			TotalAmount:    row.TotalAmount,
			CreatedAt:      row.CreatedAt,
			ItemCount:      len(items),
			FirstItemName:  firstName,
			FirstItemImage: firstImage,
		},
		Subtotal:    row.Subtotal,
		DeliveryFee: row.DeliveryFee,
		Discount:    row.Discount,
		Charges:     buildCharges(row),
		Items:       views,
		Address:     addr,
		Timeline:    s.loadTimeline(ctx, row),
		Cancellable: cancellable,
	}, nil
}

// buildCharges turns a CustomerOrderRow's pricing fields into an ordered
// breakdown ready for invoice rendering. A label appears even when its amount
// is zero so the customer can see at a glance that, e.g., shipping was free.
// For waived charges we surface the default "would have cost" so the customer
// can see the saving (e.g. ₹40 struck through next to "Free").
func buildCharges(row *repository.CustomerOrderRow) []ChargeLine {
	line := func(label string, actual, fallbackOriginal int64) ChargeLine {
		if actual > 0 {
			return ChargeLine{Label: label, Paise: actual, OriginalPaise: actual, Struck: false}
		}
		return ChargeLine{Label: label, Paise: 0, OriginalPaise: fallbackOriginal, Struck: true}
	}

	// Subtotal and GST never qualify as "free" — they're statutory or
	// arithmetic facts of the order. Always render the actual value.
	lines := []ChargeLine{
		{Label: "Subtotal", Paise: row.Subtotal, OriginalPaise: row.Subtotal, Struck: false},
		{Label: "GST", Paise: row.GST, OriginalPaise: row.GST, Struck: false},
		line("Packing", row.Packing, defaultPackingPaise),
		line("Handling", row.Handling, defaultHandlingPaise),
		line("Shipping", row.DeliveryFee, defaultShippingPaise),
		line("Surge", row.Surge, defaultSurgePaise),
		line("Platform fee", row.Platform, defaultPlatformPaise),
	}
	if row.Discount > 0 {
		lines = append(lines, ChargeLine{Label: "Discount", Paise: -row.Discount, OriginalPaise: row.Discount, Struck: false})
	}
	if row.CodRound > 0 {
		lines = append(lines, ChargeLine{Label: "Rounding (COD)", Paise: row.CodRound, OriginalPaise: row.CodRound, Struck: false})
	}
	return lines
}

// loadTimeline reads order_events rows for the order. Falls back to a synthetic
// 2-event timeline (created + current status) if the table is empty — keeps
// older orders renderable even before they were event-instrumented.
func (s *shopOrderService) loadTimeline(ctx context.Context, row *repository.CustomerOrderRow) []OrderEvent {
	events, err := s.repo.ListEvents(ctx, row.ID)
	if err != nil || len(events) == 0 {
		return []OrderEvent{
			{At: row.CreatedAt, Status: "placed"},
			{At: row.UpdatedAt, Status: row.Status},
		}
	}
	out := make([]OrderEvent, 0, len(events))
	for _, e := range events {
		out = append(out, OrderEvent{At: e.CreatedAt, Status: e.Status, Note: e.Note})
	}
	return out
}

// Cancel cancels a customer order. COD (unpaid) cancels immediately and restores
// stock in the same transaction. Razorpay (paid) path is implemented in Task 6.
func (s *shopOrderService) Cancel(ctx context.Context, customerID, orderID uuid.UUID) (*CancelResult, error) {
	// Peek at the order to decide newStatus before opening the tx.
	row, _, err := s.repo.GetByCustomerAndID(ctx, customerID, orderID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	if row.Status != "pending" && row.Status != "confirmed" {
		return nil, ErrCancelNotAllowed
	}

	newStatus := "cancelled"
	if row.PaymentStatus == "paid" {
		newStatus = "cancelling"
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	snap, err := s.repo.CancelByCustomer(tx, customerID, orderID, newStatus)
	if err != nil {
		// Either order was deleted OR status moved out of {pending,confirmed} between
		// peek and tx. Both surface to the customer as "can't cancel", but log distinguishes.
		zap.L().Info("cancel raced with concurrent state transition",
			zap.String("order_id", orderID.String()),
			zap.String("customer_id", customerID.String()))
		return nil, ErrCancelNotAllowed
	}

	if err := s.repo.RestoreStock(ctx, tx, snap.Items, orderID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Append timeline event for the cancel transition.
	if evErr := s.repo.AppendEvent(ctx, orderID, newStatus, ""); evErr != nil {
		zap.L().Warn("order_event cancel append failed",
			zap.String("order_id", orderID.String()), zap.Error(evErr))
	}

	// Razorpay path: async refund goroutine.
	if row.PaymentStatus == "paid" {
		if snap.PaymentID == nil {
			zap.L().Error("paid order has no payment_id — cannot refund",
				zap.String("order_id", orderID.String()))
			return &CancelResult{Status: "cancelling", RefundQueued: false}, nil
		}
		go func(pid uuid.UUID, amount int64) {
			rctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := s.refunder.Refund(rctx, s.shopOrgID, pid, amount, "customer_cancel"); err != nil {
				zap.L().Error("refund failed for cancelled order",
					zap.String("order_id", orderID.String()),
					zap.String("payment_id", pid.String()),
					zap.Error(err))
				// TODO(plan2c-followup): write audit_logs row via audit_log_repository,
				//                       emit metric shop.refund.cancel.failed for alerting,
				//                       enqueue retry via stuck-payment reconciliation cron.
				//                       Order stays in 'cancelling' until manual reconcile.
			}
		}(*snap.PaymentID, snap.TotalAmount)
		return &CancelResult{Status: "cancelling", RefundQueued: true, EstimatedDays: 7}, nil
	}

	return &CancelResult{Status: "cancelled", RefundQueued: false}, nil
}

// AdminList returns b2c orders for the shop org, newest first, optionally
// filtered by status. Org-scoped — not tied to any single customer.
func (s *shopOrderService) AdminList(ctx context.Context, status string, limit, offset int) ([]AdminOrderCard, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}
	args := []any{s.shopOrgID}
	q := `
		SELECT o.id, COALESCE(o.invoice_number,''), COALESCE(c.name,''), COALESCE(c.phone,''),
		       o.status, o.payment_status, COALESCE(o.payment_method,''),
		       o.total_amount,
		       (SELECT COUNT(*) FROM order_items oi WHERE oi.order_id = o.id),
		       o.created_at
		  FROM orders o
		  LEFT JOIN customers c ON c.id = o.customer_id
		 WHERE o.org_id = $1 AND o.order_type = 'b2c'`
	if status != "" {
		args = append(args, status)
		q += ` AND o.status = $2`
	}
	args = append(args, limit, offset)
	q += ` ORDER BY o.created_at DESC LIMIT $` + itoa(len(args)-1) + ` OFFSET $` + itoa(len(args))

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AdminOrderCard{}
	for rows.Next() {
		var c AdminOrderCard
		if err := rows.Scan(
			&c.ID, &c.InvoiceNumber, &c.CustomerName, &c.CustomerPhone,
			&c.Status, &c.PaymentStatus, &c.PaymentMethod,
			&c.TotalAmount, &c.ItemCount, &c.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// AdvanceStatus applies a b2c state-machine transition inside a tx, stamps the
// matching timestamp column, appends a timeline event, and returns the owning
// customer id for notification.
func (s *shopOrderService) AdvanceStatus(ctx context.Context, orderID uuid.UUID, nextStatus string) (uuid.UUID, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var current string
	var customerID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT status, customer_id FROM orders
		 WHERE id = $1 AND org_id = $2 AND order_type = 'b2c'
		 FOR UPDATE`, orderID, s.shopOrgID).Scan(&current, &customerID)
	if err != nil {
		return uuid.Nil, ErrOrderNotFound
	}
	if !canAdvance(current, nextStatus) {
		return uuid.Nil, ErrInvalidTransition
	}

	// Stamp the lifecycle timestamp matching the new status (best-effort columns).
	tsCol := ""
	switch nextStatus {
	case "shipped":
		tsCol = "shipped_at"
	case "delivered":
		tsCol = "delivered_at"
	case "cancelled":
		tsCol = "cancelled_at"
	}
	set := "status = $1, updated_at = NOW()"
	if tsCol != "" {
		set += ", " + tsCol + " = NOW()"
	}
	if _, err := tx.Exec(ctx,
		`UPDATE orders SET `+set+` WHERE id = $2 AND org_id = $3`,
		nextStatus, orderID, s.shopOrgID); err != nil {
		return uuid.Nil, err
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO order_events (order_id, status, note) VALUES ($1, $2, '')`,
		orderID, nextStatus); err != nil {
		return uuid.Nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return customerID, nil
}

// itoa avoids importing strconv just for parameter indexing.
func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
