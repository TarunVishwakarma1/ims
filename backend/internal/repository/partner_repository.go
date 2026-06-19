package repository

import (
	"context"
	"errors"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PartnerRepository aggregates counterparties from the orders table.
// Suppliers = orgs that sold to us; Buyers = orgs that bought from us.
type PartnerRepository interface {
	// ListByRole returns every distinct partner of the given role for the
	// caller org. role ∈ {"supplier","buyer"}.
	ListByRole(ctx context.Context, callerOrgID uuid.UUID, role string) ([]*domain.Partner, error)

	// GetRelationship returns the partner aggregate for ONE counterparty in
	// ONE direction. Used to fill PartnerDetail.AsSupplier / AsBuyer.
	GetRelationship(ctx context.Context, callerOrgID, partnerOrgID uuid.UUID, role string) (*domain.Partner, error)

	// RecentOrdersWith returns the latest N orders that flow between caller
	// and partner in EITHER direction.
	RecentOrdersWith(ctx context.Context, callerOrgID, partnerOrgID uuid.UUID, limit int) ([]*domain.Order, error)
}

type partnerRepository struct {
	db DBTX
}

func NewPartnerRepository(db DBTX) PartnerRepository {
	return &partnerRepository{db: db}
}

// columnsFor maps a role onto the (partner-org column, caller-org column).
//
//	supplier → partner = supplier_org_id, caller = buyer_org_id
//	buyer    → partner = buyer_org_id,    caller = supplier_org_id
func columnsFor(role string) (partnerCol, callerCol string, err error) {
	switch role {
	case "supplier":
		return "supplier_org_id", "buyer_org_id", nil
	case "buyer":
		return "buyer_org_id", "supplier_org_id", nil
	default:
		return "", "", errors.New("invalid partner role: " + role)
	}
}

const partnerActiveStatuses = `('pending','confirmed','accepted','processing','ready','shipped')`

func (r *partnerRepository) ListByRole(ctx context.Context, callerOrgID uuid.UUID, role string) ([]*domain.Partner, error) {
	partnerCol, callerCol, err := columnsFor(role)
	if err != nil {
		return nil, err
	}
	q := `
		SELECT o.id, o.name, o.slug,
		       COUNT(*) AS order_count,
		       COALESCE(SUM(orders.total_amount), 0) AS total_value,
		       COALESCE(SUM(CASE WHEN orders.status IN ` + partnerActiveStatuses + ` THEN 1 ELSE 0 END), 0) AS pending_count,
		       MAX(orders.created_at) AS last_order_at
		FROM orders
		JOIN organizations o ON o.id = orders.` + partnerCol + `
		WHERE orders.` + callerCol + ` = $1
		  AND orders.` + partnerCol + ` IS NOT NULL
		GROUP BY o.id, o.name, o.slug
		ORDER BY last_order_at DESC NULLS LAST
	`
	rows, err := r.db.Query(ctx, q, callerOrgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.Partner
	for rows.Next() {
		p := &domain.Partner{Role: role}
		if err := rows.Scan(&p.OrgID, &p.Name, &p.Slug, &p.OrderCount, &p.TotalValue, &p.PendingCount, &p.LastOrderAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *partnerRepository) GetRelationship(ctx context.Context, callerOrgID, partnerOrgID uuid.UUID, role string) (*domain.Partner, error) {
	partnerCol, callerCol, err := columnsFor(role)
	if err != nil {
		return nil, err
	}
	q := `
		SELECT o.id, o.name, o.slug,
		       COUNT(*) AS order_count,
		       COALESCE(SUM(orders.total_amount), 0) AS total_value,
		       COALESCE(SUM(CASE WHEN orders.status IN ` + partnerActiveStatuses + ` THEN 1 ELSE 0 END), 0) AS pending_count,
		       MAX(orders.created_at) AS last_order_at
		FROM orders
		JOIN organizations o ON o.id = orders.` + partnerCol + `
		WHERE orders.` + callerCol + ` = $1 AND orders.` + partnerCol + ` = $2
		GROUP BY o.id, o.name, o.slug
	`
	row := r.db.QueryRow(ctx, q, callerOrgID, partnerOrgID)
	p := &domain.Partner{Role: role}
	if err := row.Scan(&p.OrgID, &p.Name, &p.Slug, &p.OrderCount, &p.TotalValue, &p.PendingCount, &p.LastOrderAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

// RecentOrdersWith fetches recent orders + the buyer/supplier org names in a
// single JOIN. Avoids the N+1 of looping over orders to load org names
// individually. Names are inlined onto Order via the optional display
// fields; UUID-only callers keep working because the JSON tag uses
// `omitempty`.
func (r *partnerRepository) RecentOrdersWith(ctx context.Context, callerOrgID, partnerOrgID uuid.UUID, limit int) ([]*domain.Order, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	// orderCols are aliased to `o.` for the JOIN.
	q := `SELECT ` + qualifyOrderCols("o") + `,
			COALESCE(bo.name, '') AS buyer_name,
			COALESCE(so.name, '') AS supplier_name
		FROM orders o
		LEFT JOIN organizations bo ON bo.id = o.buyer_org_id
		LEFT JOIN organizations so ON so.id = o.supplier_org_id
		WHERE (
			(o.buyer_org_id = $1 AND o.supplier_org_id = $2) OR
			(o.buyer_org_id = $2 AND o.supplier_org_id = $1)
		)
		ORDER BY o.created_at DESC
		LIMIT $3`
	rows, err := r.db.Query(ctx, q, callerOrgID, partnerOrgID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.Order
	for rows.Next() {
		o := &domain.Order{}
		err := rows.Scan(
			&o.ID, &o.OrgID, &o.UserID, &o.Status, &o.TotalAmount, &o.CreatedAt, &o.UpdatedAt,
			&o.OrderType, &o.BuyerOrgID, &o.SupplierOrgID, &o.SupplierLocationID, &o.CustomerID,
			&o.DeliveryAddressID, &o.Subtotal, &o.DeliveryFee, &o.Discount,
			&o.PaymentStatus, &o.PaymentID,
			&o.AcceptedAt, &o.ShippedAt, &o.DeliveredAt, &o.CompletedAt, &o.CancelledAt,
			&o.BuyerOrgName, &o.SupplierOrgName,
		)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// qualifyOrderCols prefixes every column in orderCols with the given alias
// so it can be used inside a JOIN without ambiguity.
func qualifyOrderCols(alias string) string {
	// orderCols is a comma+whitespace list; rebuild with the alias prefix.
	parts := []string{
		"id", "org_id", "user_id", "status", "total_amount", "created_at", "updated_at",
		"order_type", "buyer_org_id", "supplier_org_id", "supplier_location_id", "customer_id",
		"delivery_address_id", "subtotal", "delivery_fee", "discount",
		"payment_status", "payment_id",
		"accepted_at", "shipped_at", "delivered_at", "completed_at", "cancelled_at",
	}
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += alias + "." + p
	}
	return out
}
