package repository

import (
	"context"
	"errors"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CustomerAddressRepository handles CRUD for customer_addresses.
type CustomerAddressRepository interface {
	Create(ctx context.Context, a *domain.CustomerAddress) (uuid.UUID, error)
	ListByCustomer(ctx context.Context, customerID uuid.UUID) ([]domain.CustomerAddress, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.CustomerAddress, error)
	Update(ctx context.Context, a *domain.CustomerAddress) error
	Delete(ctx context.Context, id, customerID uuid.UUID) error
	// SetDefault atomically clears all other defaults for the customer then
	// marks the given address as default, inside a single transaction.
	SetDefault(ctx context.Context, id, customerID uuid.UUID) error
	WithTx(tx pgx.Tx) CustomerAddressRepository
}

type customerAddressRepository struct {
	db   DBTX
	pool *pgxpool.Pool
}

// NewCustomerAddressRepository returns a CustomerAddressRepository backed by pool.
func NewCustomerAddressRepository(pool *pgxpool.Pool) CustomerAddressRepository {
	return &customerAddressRepository{db: pool, pool: pool}
}

// WithTx returns a new repo that executes within the provided transaction.
func (r *customerAddressRepository) WithTx(tx pgx.Tx) CustomerAddressRepository {
	return &customerAddressRepository{db: tx, pool: r.pool}
}

// addrScanCols is the canonical column list for SELECT queries.
const addrScanCols = `id, customer_id,
	COALESCE(label,''),
	COALESCE(name,''),
	COALESCE(phone,''),
	line1,
	COALESCE(line2,''),
	COALESCE(city,''),
	COALESCE(state,''),
	COALESCE(country,''),
	COALESCE(postal_code,''),
	lat, lng, is_default, created_at`

func scanAddress(row interface{ Scan(dest ...any) error }) (*domain.CustomerAddress, error) {
	a := &domain.CustomerAddress{}
	err := row.Scan(
		&a.ID, &a.CustomerID,
		&a.Label, &a.Name, &a.Phone, &a.Line1, &a.Line2,
		&a.City, &a.State, &a.Country, &a.PostalCode,
		&a.Lat, &a.Lng, &a.IsDefault, &a.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// Create inserts a new address. If IsDefault is true, all existing defaults for
// the customer are cleared first within a single transaction.
func (r *customerAddressRepository) Create(ctx context.Context, a *domain.CustomerAddress) (uuid.UUID, error) {
	if a.IsDefault {
		// Need a transaction to clear existing defaults and insert atomically.
		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return uuid.Nil, err
		}
		defer tx.Rollback(ctx) //nolint:errcheck

		if _, err = tx.Exec(ctx,
			`UPDATE customer_addresses SET is_default = FALSE WHERE customer_id = $1`,
			a.CustomerID,
		); err != nil {
			return uuid.Nil, err
		}

		var id uuid.UUID
		err = tx.QueryRow(ctx, `
			INSERT INTO customer_addresses
				(customer_id, label, name, phone, line1, line2, city, state,
				 country, postal_code, lat, lng, is_default)
			VALUES
				($1, $2, $3, $4, $5, $6, $7, $8,
				 COALESCE(NULLIF($9,''),'IN'), $10, $11, $12, $13)
			RETURNING id
		`,
			a.CustomerID, a.Label, a.Name, a.Phone, a.Line1, a.Line2, a.City, a.State,
			a.Country, a.PostalCode, a.Lat, a.Lng, a.IsDefault,
		).Scan(&id)
		if err != nil {
			return uuid.Nil, err
		}
		return id, tx.Commit(ctx)
	}

	// No-default fast path — single statement, no tx needed.
	var id uuid.UUID
	err := r.db.QueryRow(ctx, `
		INSERT INTO customer_addresses
			(customer_id, label, name, phone, line1, line2, city, state,
			 country, postal_code, lat, lng, is_default)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8,
			 COALESCE(NULLIF($9,''),'IN'), $10, $11, $12, $13)
		RETURNING id
	`,
		a.CustomerID, a.Label, a.Name, a.Phone, a.Line1, a.Line2, a.City, a.State,
		a.Country, a.PostalCode, a.Lat, a.Lng, a.IsDefault,
	).Scan(&id)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// ListByCustomer returns all addresses for a customer, defaults first.
func (r *customerAddressRepository) ListByCustomer(ctx context.Context, customerID uuid.UUID) ([]domain.CustomerAddress, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+addrScanCols+`
		FROM customer_addresses
		WHERE customer_id = $1
		ORDER BY is_default DESC, created_at DESC
	`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.CustomerAddress
	for rows.Next() {
		a, err := scanAddress(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

// GetByID returns the address with the given ID, or nil if not found.
func (r *customerAddressRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.CustomerAddress, error) {
	a, err := scanAddress(r.db.QueryRow(ctx, `
		SELECT `+addrScanCols+`
		FROM customer_addresses
		WHERE id = $1
	`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

// Update changes label, line1, line2, city, state, country, postal_code, lat, lng
// for the address identified by both id AND customerID (scoped update).
func (r *customerAddressRepository) Update(ctx context.Context, a *domain.CustomerAddress) error {
	_, err := r.db.Exec(ctx, `
		UPDATE customer_addresses
		SET
			label       = $2,
			name        = $3,
			phone       = $4,
			line1       = $5,
			line2       = $6,
			city        = $7,
			state       = $8,
			country     = COALESCE(NULLIF($9,''),'IN'),
			postal_code = $10,
			lat         = $11,
			lng         = $12
		WHERE id = $1 AND customer_id = $13
	`,
		a.ID, a.Label, a.Name, a.Phone, a.Line1, a.Line2, a.City, a.State,
		a.Country, a.PostalCode, a.Lat, a.Lng,
		a.CustomerID,
	)
	return err
}

// Delete removes the address identified by both id AND customerID.
func (r *customerAddressRepository) Delete(ctx context.Context, id, customerID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM customer_addresses WHERE id = $1 AND customer_id = $2`,
		id, customerID,
	)
	return err
}

// SetDefault atomically clears all existing defaults for customerID then marks id
// as default. Runs both updates inside a single transaction.
func (r *customerAddressRepository) SetDefault(ctx context.Context, id, customerID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err = tx.Exec(ctx,
		`UPDATE customer_addresses SET is_default = FALSE WHERE customer_id = $1`,
		customerID,
	); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx,
		`UPDATE customer_addresses SET is_default = TRUE WHERE id = $1 AND customer_id = $2`,
		id, customerID,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
