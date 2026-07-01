package shop

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SalesDay is one calendar day of b2c sales for a shop.
type SalesDay struct {
	Date         string `json:"date"` // YYYY-MM-DD
	Orders       int    `json:"orders"`
	RevenuePaise int64  `json:"revenue_paise"`
}

// SalesSummary aggregates a shop's non-cancelled b2c orders over a window.
type SalesSummary struct {
	Days          int        `json:"days"`
	Orders        int        `json:"orders"`
	RevenuePaise  int64      `json:"revenue_paise"`
	AvgOrderPaise int64      `json:"avg_order_paise"`
	ByDay         []SalesDay `json:"by_day"`
}

// ShopAnalyticsService reports a single shop's sales, scoped to its org.
type ShopAnalyticsService interface {
	SalesSummary(ctx context.Context, orgID uuid.UUID, days int) (*SalesSummary, error)
}

type shopAnalyticsService struct {
	pool *pgxpool.Pool
}

func NewShopAnalyticsService(pool *pgxpool.Pool) ShopAnalyticsService {
	return &shopAnalyticsService{pool: pool}
}

func (s *shopAnalyticsService) SalesSummary(ctx context.Context, orgID uuid.UUID, days int) (*SalesSummary, error) {
	if days <= 0 || days > 365 {
		days = 30
	}

	rows, err := s.pool.Query(ctx, `
		SELECT to_char(created_at::date, 'YYYY-MM-DD') AS d,
		       COUNT(*)                                 AS orders,
		       COALESCE(SUM(total_amount), 0)           AS revenue
		  FROM orders
		 WHERE org_id = $1
		   AND order_type = 'b2c'
		   AND status NOT IN ('cancelled', 'cancelling')
		   AND created_at >= NOW() - make_interval(days => $2)
		 GROUP BY d
		 ORDER BY d`, orgID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sum := &SalesSummary{Days: days, ByDay: []SalesDay{}}
	for rows.Next() {
		var d SalesDay
		if err := rows.Scan(&d.Date, &d.Orders, &d.RevenuePaise); err != nil {
			return nil, err
		}
		sum.Orders += d.Orders
		sum.RevenuePaise += d.RevenuePaise
		sum.ByDay = append(sum.ByDay, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if sum.Orders > 0 {
		sum.AvgOrderPaise = sum.RevenuePaise / int64(sum.Orders)
	}
	return sum, nil
}
