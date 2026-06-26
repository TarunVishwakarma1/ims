package shop

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
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

// ChargeLine is a single row in the invoice price breakdown. Struck=true asks
// the UI to render the amount with a strikethrough + "Free" label.
type ChargeLine struct {
	Label  string `json:"label"`
	Paise  int64  `json:"paise"`
	Struck bool   `json:"struck"`
}

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

type ShopOrderService interface {
	List(ctx context.Context, customerID uuid.UUID, q OrderListQuery) (*OrderListResult, error)
	Get(ctx context.Context, customerID, orderID uuid.UUID) (*OrderDetail, error)
	Cancel(ctx context.Context, customerID, orderID uuid.UUID) (*CancelResult, error)
}

var (
	ErrOrderNotFound    = errors.New("order not found")
	ErrCancelNotAllowed = errors.New("order status does not allow cancel")
	ErrRefundFailed     = errors.New("refund request failed")
	ErrInvalidCursor    = errors.New("invalid cursor")
)

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
func buildCharges(row *repository.CustomerOrderRow) []ChargeLine {
	lines := []ChargeLine{
		{Label: "Subtotal", Paise: row.Subtotal, Struck: row.Subtotal == 0},
		{Label: "GST", Paise: row.GST, Struck: row.GST == 0},
		{Label: "Packing", Paise: row.Packing, Struck: row.Packing == 0},
		{Label: "Handling", Paise: row.Handling, Struck: row.Handling == 0},
		{Label: "Shipping", Paise: row.DeliveryFee, Struck: row.DeliveryFee == 0},
		{Label: "Surge", Paise: row.Surge, Struck: row.Surge == 0},
	}
	if row.Discount > 0 {
		lines = append(lines, ChargeLine{Label: "Discount", Paise: -row.Discount, Struck: false})
	}
	if row.CodRound > 0 {
		lines = append(lines, ChargeLine{Label: "Rounding (COD)", Paise: row.CodRound, Struck: false})
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
