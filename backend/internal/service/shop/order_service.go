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
	TotalAmount    int64     `json:"total_amount_paise"`
	CreatedAt      time.Time `json:"created_at"`
	ItemCount      int       `json:"item_count"`
	FirstItemName  string    `json:"first_item_name"`
	FirstItemImage string    `json:"first_item_image,omitempty"`
}

type OrderDetail struct {
	OrderCard
	Subtotal    int64                  `json:"subtotal_paise"`
	DeliveryFee int64                  `json:"delivery_fee_paise"`
	Discount    int64                  `json:"discount_paise"`
	Items       []OrderItemView        `json:"items"`
	Address     map[string]interface{} `json:"delivery_address"`
	Timeline    []OrderEvent           `json:"timeline"`
	Cancellable bool                   `json:"cancellable"`
}

type OrderItemView struct {
	ProductID uuid.UUID `json:"product_id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	ImageURL  string    `json:"image_url"`
	Quantity  int       `json:"quantity"`
	UnitPrice int64     `json:"unit_price_paise"`
}

type OrderEvent struct {
	At     time.Time `json:"at"`
	Status string    `json:"status"`
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

func decodeOrderCursor(s string) (time.Time, uuid.UUID, bool) {
	if s == "" {
		return time.Time{}, uuid.Nil, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}
	var p orderCursorPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return time.Time{}, uuid.Nil, false
	}
	return p.K, p.I, true
}

func (s *shopOrderService) List(ctx context.Context, customerID uuid.UUID, q OrderListQuery) (*OrderListResult, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	cursorAt, cursorID, _ := decodeOrderCursor(q.Cursor)

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
			ImageURL:  it.ProductImage,
			Quantity:  it.Quantity,
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
		Items:       views,
		Address:     addr,
		Timeline:    []OrderEvent{{At: row.CreatedAt, Status: "created"}, {At: row.UpdatedAt, Status: row.Status}},
		Cancellable: cancellable,
	}, nil
}

// Cancel cancels a customer order. COD (unpaid) cancels immediately and restores
// stock in the same transaction. Razorpay (paid) path is implemented in Task 6.
func (s *shopOrderService) Cancel(ctx context.Context, customerID, orderID uuid.UUID) (*CancelResult, error) {
	// Peek at the order to decide newStatus before opening the tx.
	row, _, err := s.repo.GetByCustomerAndID(ctx, customerID, orderID)
	if err != nil {
		return nil, ErrOrderNotFound
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
		// Concurrent state transition lost the race.
		return nil, ErrCancelNotAllowed
	}

	if err := s.repo.RestoreStock(ctx, tx, snap.Items, orderID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Razorpay path: Task 6.
	if row.PaymentStatus == "paid" {
		return nil, fmt.Errorf("paid cancel not yet implemented")
	}

	return &CancelResult{Status: "cancelled", RefundQueued: false}, nil
}
