package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type MarketplaceRepository interface {
	// Listings
	CreateListing(ctx context.Context, listing *domain.MarketplaceListing) error
	UpdateListing(ctx context.Context, listing *domain.MarketplaceListing) error
	DeleteListing(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error
	GetListingByID(ctx context.Context, id uuid.UUID) (*domain.MarketplaceListing, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]*domain.MarketplaceListing, error)

	// Search
	Search(ctx context.Context, query string, lat, lng, radiusKM *float64, filters map[string]any) ([]*domain.MarketplaceListing, error)

	// Carts
	CreateCart(ctx context.Context, cart *domain.Cart) error
	GetActiveCart(ctx context.Context, buyerOrgID, customerID *uuid.UUID) (*domain.Cart, error)
	AddCartItem(ctx context.Context, item *domain.CartItem) error
	UpdateCartItem(ctx context.Context, item *domain.CartItem) error
	RemoveCartItem(ctx context.Context, cartID, listingID uuid.UUID) error
	GetCartWithItems(ctx context.Context, cartID uuid.UUID) (*domain.Cart, error)

	// Reservations
	CreateReservation(ctx context.Context, res *domain.InventoryReservation) error
	ReleaseReservation(ctx context.Context, id uuid.UUID) error
	CommitReservation(ctx context.Context, id uuid.UUID) error
	ExpireStaleReservations(ctx context.Context) error
	// ReleaseByOrder marks all of an order's still-committed reservations as
	// released. Returns the rows so the caller can restore inventory amounts.
	ReleaseByOrder(ctx context.Context, orderID uuid.UUID) ([]*domain.InventoryReservation, error)

	WithTx(tx pgx.Tx) MarketplaceRepository
}

type marketplaceRepository struct {
	db DBTX
}

func NewMarketplaceRepository(db DBTX) MarketplaceRepository {
	return &marketplaceRepository{db: db}
}

func (r *marketplaceRepository) WithTx(tx pgx.Tx) MarketplaceRepository {
	return &marketplaceRepository{db: tx}
}

// --- Listings ---

func (r *marketplaceRepository) CreateListing(ctx context.Context, listing *domain.MarketplaceListing) error {
	query := `
		INSERT INTO marketplace_listings (id, org_id, product_id, location_id, listing_price, min_order_qty, max_order_qty, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.Exec(ctx, query,
		listing.ID, listing.OrgID, listing.ProductID, listing.LocationID, listing.ListingPrice,
		listing.MinOrderQty, listing.MaxOrderQty, listing.IsActive, listing.CreatedAt, listing.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrConflict
		}
		return err
	}
	return nil
}

func (r *marketplaceRepository) UpdateListing(ctx context.Context, listing *domain.MarketplaceListing) error {
	query := `
		UPDATE marketplace_listings
		SET listing_price = $1,
		    min_order_qty = $2,
		    max_order_qty = $3,
		    is_active     = $4,
		    location_id   = $5,
		    updated_at    = $6
		WHERE id = $7 AND org_id = $8
	`
	res, err := r.db.Exec(ctx, query,
		listing.ListingPrice, listing.MinOrderQty, listing.MaxOrderQty,
		listing.IsActive, listing.LocationID, listing.UpdatedAt,
		listing.ID, listing.OrgID,
	)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *marketplaceRepository) DeleteListing(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error {
	query := `DELETE FROM marketplace_listings WHERE id = $1 AND org_id = $2`
	res, err := r.db.Exec(ctx, query, id, orgID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *marketplaceRepository) GetListingByID(ctx context.Context, id uuid.UUID) (*domain.MarketplaceListing, error) {
	query := `
		SELECT id, org_id, product_id, location_id, listing_price, min_order_qty, max_order_qty, is_active, created_at, updated_at
		FROM marketplace_listings WHERE id = $1
	`
	var l domain.MarketplaceListing
	err := r.db.QueryRow(ctx, query, id).Scan(
		&l.ID, &l.OrgID, &l.ProductID, &l.LocationID, &l.ListingPrice,
		&l.MinOrderQty, &l.MaxOrderQty, &l.IsActive, &l.CreatedAt, &l.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &l, nil
}

func (r *marketplaceRepository) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]*domain.MarketplaceListing, error) {
	query := `
		SELECT m.id, m.org_id, m.product_id, m.location_id, m.listing_price, m.min_order_qty, m.max_order_qty, m.is_active, m.created_at, m.updated_at,
		       p.name, p.sku, o.name, o.slug, l.name, l.city
		FROM marketplace_listings m
		JOIN products p ON m.product_id = p.id
		JOIN organizations o ON m.org_id = o.id
		LEFT JOIN org_locations l ON m.location_id = l.id
		WHERE m.org_id = $1
		ORDER BY m.created_at DESC
	`
	rows, err := r.db.Query(ctx, query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var listings []*domain.MarketplaceListing
	for rows.Next() {
		var m domain.MarketplaceListing
		var pname, psku, oname, oslug, lname, lcity *string
		if err := rows.Scan(
			&m.ID, &m.OrgID, &m.ProductID, &m.LocationID, &m.ListingPrice, &m.MinOrderQty, &m.MaxOrderQty, &m.IsActive, &m.CreatedAt, &m.UpdatedAt,
			&pname, &psku, &oname, &oslug, &lname, &lcity,
		); err != nil {
			return nil, err
		}
		if pname != nil { m.ProductName = *pname }
		if psku != nil { m.ProductSKU = *psku }
		if oname != nil { m.OrgName = *oname }
		if oslug != nil { m.OrgSlug = *oslug }
		if lname != nil { m.LocationName = *lname }
		if lcity != nil { m.LocationCity = *lcity }

		listings = append(listings, &m)
	}
	return listings, nil
}

// --- Search ---

func (r *marketplaceRepository) Search(ctx context.Context, query string, lat, lng, radiusKM *float64, filters map[string]any) ([]*domain.MarketplaceListing, error) {
	var args []any
	argCount := 1

	selectClause := `
		SELECT m.id, m.org_id, m.product_id, m.location_id, m.listing_price, m.min_order_qty, m.max_order_qty, m.is_active, m.created_at, m.updated_at,
		       p.name, p.sku, o.name, o.slug, l.name, l.city, l.lat, l.lng,
			   COALESCE(SUM(i.quantity), 0) as stock_quantity
	`
	fromClause := `
		FROM marketplace_listings m
		JOIN products p ON m.product_id = p.id
		JOIN organizations o ON m.org_id = o.id
		LEFT JOIN org_locations l ON m.location_id = l.id
		LEFT JOIN inventory i ON p.id = i.product_id AND o.id = i.org_id
	`
	
	var whereClauses []string
	whereClauses = append(whereClauses, "m.is_active = true")

	// Full-Text Search
	if query != "" {
		// Convert query to websearch_to_tsquery format
		whereClauses = append(whereClauses, fmt.Sprintf("p.search_vector @@ websearch_to_tsquery('english', $%d)", argCount))
		args = append(args, query)
		argCount++
		
		// Order by rank
		selectClause += fmt.Sprintf(", ts_rank(p.search_vector, websearch_to_tsquery('english', $%d)) as rank", argCount-1)
	} else {
		selectClause += ", 0.0 as rank"
	}

	// Geolocation Haversine
	if lat != nil && lng != nil && radiusKM != nil {
		distSql := fmt.Sprintf(`
			(6371 * ACOS(
				COS(RADIANS($%d)) * COS(RADIANS(l.lat)) * COS(RADIANS(l.lng) - RADIANS($%d)) +
				SIN(RADIANS($%d)) * SIN(RADIANS(l.lat))
			))`, argCount, argCount+1, argCount)
		
		selectClause += fmt.Sprintf(", %s as distance_km", distSql)
		// Include global listings (no location) when searching by radius
		whereClauses = append(whereClauses, fmt.Sprintf("(%s <= $%d OR l.id IS NULL)", distSql, argCount+2))
		
		args = append(args, *lat, *lng, *radiusKM)
		argCount += 3
	} else {
		selectClause += ", NULL::float as distance_km"
	}

	// Filters
	if minPrice, ok := filters["min_price"]; ok {
		whereClauses = append(whereClauses, fmt.Sprintf("m.listing_price >= $%d", argCount))
		args = append(args, minPrice)
		argCount++
	}
	if maxPrice, ok := filters["max_price"]; ok {
		whereClauses = append(whereClauses, fmt.Sprintf("m.listing_price <= $%d", argCount))
		args = append(args, maxPrice)
		argCount++
	}

	finalQuery := selectClause + fromClause + " WHERE " + strings.Join(whereClauses, " AND ") + 
		` GROUP BY m.id, p.id, o.id, l.id `
	
	if query != "" {
		finalQuery += " ORDER BY rank DESC, distance_km ASC NULLS LAST"
	} else if lat != nil && lng != nil {
		finalQuery += " ORDER BY distance_km ASC NULLS LAST"
	} else {
		finalQuery += " ORDER BY m.created_at DESC"
	}

	finalQuery += " LIMIT 100"

	rows, err := r.db.Query(ctx, finalQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	listings := make([]*domain.MarketplaceListing, 0)
	for rows.Next() {
		var m domain.MarketplaceListing
		var pname, psku, oname, oslug, lname, lcity *string
		var rank float64 // Ignored in struct, used for sorting
		if err := rows.Scan(
			&m.ID, &m.OrgID, &m.ProductID, &m.LocationID, &m.ListingPrice, &m.MinOrderQty, &m.MaxOrderQty, &m.IsActive, &m.CreatedAt, &m.UpdatedAt,
			&pname, &psku, &oname, &oslug, &lname, &lcity, &m.LocationLat, &m.LocationLng, &m.StockQuantity, &rank, &m.DistanceKM,
		); err != nil {
			return nil, err
		}
		if pname != nil { m.ProductName = *pname }
		if psku != nil { m.ProductSKU = *psku }
		if oname != nil { m.OrgName = *oname }
		if oslug != nil { m.OrgSlug = *oslug }
		if lname != nil { m.LocationName = *lname }
		if lcity != nil { m.LocationCity = *lcity }

		listings = append(listings, &m)
	}
	return listings, nil
}

// --- Carts ---

func (r *marketplaceRepository) CreateCart(ctx context.Context, cart *domain.Cart) error {
	query := `
		INSERT INTO carts (id, buyer_org_id, customer_id, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.db.Exec(ctx, query, cart.ID, cart.BuyerOrgID, cart.CustomerID, cart.ExpiresAt, cart.CreatedAt)
	return err
}

func (r *marketplaceRepository) GetActiveCart(ctx context.Context, buyerOrgID, customerID *uuid.UUID) (*domain.Cart, error) {
	query := `
		SELECT id, buyer_org_id, customer_id, expires_at, created_at
		FROM carts
		WHERE (buyer_org_id = $1 OR ($1 IS NULL AND buyer_org_id IS NULL))
		  AND (customer_id = $2 OR ($2 IS NULL AND customer_id IS NULL))
		  AND expires_at > NOW()
		ORDER BY created_at DESC LIMIT 1
	`
	var c domain.Cart
	err := r.db.QueryRow(ctx, query, buyerOrgID, customerID).Scan(
		&c.ID, &c.BuyerOrgID, &c.CustomerID, &c.ExpiresAt, &c.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *marketplaceRepository) AddCartItem(ctx context.Context, item *domain.CartItem) error {
	query := `
		INSERT INTO cart_items (id, cart_id, listing_id, quantity, added_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (cart_id, listing_id) DO UPDATE SET quantity = cart_items.quantity + EXCLUDED.quantity
	`
	_, err := r.db.Exec(ctx, query, item.ID, item.CartID, item.ListingID, item.Quantity, item.AddedAt)
	return err
}

func (r *marketplaceRepository) UpdateCartItem(ctx context.Context, item *domain.CartItem) error {
	query := `UPDATE cart_items SET quantity = $1 WHERE cart_id = $2 AND listing_id = $3`
	res, err := r.db.Exec(ctx, query, item.Quantity, item.CartID, item.ListingID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *marketplaceRepository) RemoveCartItem(ctx context.Context, cartID, listingID uuid.UUID) error {
	query := `DELETE FROM cart_items WHERE cart_id = $1 AND listing_id = $2`
	_, err := r.db.Exec(ctx, query, cartID, listingID)
	return err
}

func (r *marketplaceRepository) GetCartWithItems(ctx context.Context, cartID uuid.UUID) (*domain.Cart, error) {
	// First get cart
	queryCart := `SELECT id, buyer_org_id, customer_id, expires_at, created_at FROM carts WHERE id = $1`
	var c domain.Cart
	err := r.db.QueryRow(ctx, queryCart, cartID).Scan(&c.ID, &c.BuyerOrgID, &c.CustomerID, &c.ExpiresAt, &c.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}

	// Then get items with listings
	queryItems := `
		SELECT ci.id, ci.cart_id, ci.listing_id, ci.quantity, ci.added_at,
		       m.id, m.org_id, m.product_id, m.listing_price, p.name, o.name
		FROM cart_items ci
		JOIN marketplace_listings m ON ci.listing_id = m.id
		JOIN products p ON m.product_id = p.id
		JOIN organizations o ON m.org_id = o.id
		WHERE ci.cart_id = $1
		ORDER BY ci.added_at ASC
	`
	rows, err := r.db.Query(ctx, queryItems, cartID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item domain.CartItem
		var listing domain.MarketplaceListing
		if err := rows.Scan(
			&item.ID, &item.CartID, &item.ListingID, &item.Quantity, &item.AddedAt,
			&listing.ID, &listing.OrgID, &listing.ProductID, &listing.ListingPrice, &listing.ProductName, &listing.OrgName,
		); err != nil {
			return nil, err
		}
		item.Listing = &listing
		c.Items = append(c.Items, item)
	}
	return &c, nil
}

// --- Reservations ---

func (r *marketplaceRepository) CreateReservation(ctx context.Context, res *domain.InventoryReservation) error {
	query := `
		INSERT INTO inventory_reservations (id, inventory_id, order_id, org_id, quantity, status, reserved_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.Exec(ctx, query, res.ID, res.InventoryID, res.OrderID, res.OrgID, res.Quantity, res.Status, res.ReservedAt, res.ExpiresAt)
	return err
}

func (r *marketplaceRepository) ReleaseReservation(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE inventory_reservations SET status = 'released', released_at = NOW() WHERE id = $1 AND status = 'active'`
	res, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *marketplaceRepository) CommitReservation(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE inventory_reservations SET status = 'committed' WHERE id = $1 AND status = 'active'`
	res, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *marketplaceRepository) ExpireStaleReservations(ctx context.Context) error {
	query := `UPDATE inventory_reservations SET status = 'expired' WHERE status = 'active' AND expires_at <= NOW()`
	_, err := r.db.Exec(ctx, query)
	return err
}

func (r *marketplaceRepository) ReleaseByOrder(ctx context.Context, orderID uuid.UUID) ([]*domain.InventoryReservation, error) {
	// Atomically flip status and return the affected rows so the caller can
	// add the reserved quantity back to inventory in the same transaction.
	rows, err := r.db.Query(ctx, `
		UPDATE inventory_reservations
		SET status = 'released', released_at = NOW()
		WHERE order_id = $1 AND status IN ('active', 'committed')
		RETURNING id, inventory_id, order_id, org_id, quantity, status, reserved_at, expires_at, released_at
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*domain.InventoryReservation, 0)
	for rows.Next() {
		var r domain.InventoryReservation
		if err := rows.Scan(&r.ID, &r.InventoryID, &r.OrderID, &r.OrgID, &r.Quantity, &r.Status, &r.ReservedAt, &r.ExpiresAt, &r.ReleasedAt); err != nil {
			return nil, err
		}
		out = append(out, &r)
	}
	return out, rows.Err()
}
