package repository

import (
	"context"
	"errors"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type CustomerRepository interface {
	Create(ctx context.Context, customer *domain.Customer) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Customer, error)
	GetByEmail(ctx context.Context, email string) (*domain.Customer, error)
	GetByPhone(ctx context.Context, phone string) (*domain.Customer, error)
	// FindByPhone looks up a customer by phone. Returns (nil, nil) if not found.
	FindByPhone(ctx context.Context, phone string) (*domain.Customer, error)
	// UpsertByPhone inserts a customer with the given phone (name='', is_verified=TRUE)
	// if none exists, otherwise sets is_verified=TRUE and updated_at=NOW().
	// Always returns the full row.
	UpsertByPhone(ctx context.Context, phone string) (*domain.Customer, error)
	UpdateVerified(ctx context.Context, id uuid.UUID, isVerified bool) error
	// UpdateProfile sets the customer's name and email (email may be empty string,
	// which is stored as NULL to avoid UNIQUE collisions).
	UpdateProfile(ctx context.Context, id uuid.UUID, name, email string) error
	CreateAddress(ctx context.Context, address *domain.CustomerAddress) error
	ListAddresses(ctx context.Context, customerID uuid.UUID) ([]*domain.CustomerAddress, error)
	WithTx(tx pgx.Tx) CustomerRepository
}

type customerRepository struct {
	db DBTX
}

func NewCustomerRepository(db DBTX) CustomerRepository {
	return &customerRepository{db: db}
}

func (r *customerRepository) WithTx(tx pgx.Tx) CustomerRepository {
	return &customerRepository{db: tx}
}

func (r *customerRepository) Create(ctx context.Context, customer *domain.Customer) error {
	query := `
		INSERT INTO customers (id, name, email, phone, password_hash, is_verified, is_guest, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.Exec(ctx, query,
		customer.ID, customer.Name, customer.Email, customer.Phone, customer.PasswordHash,
		customer.IsVerified, customer.IsGuest, customer.CreatedAt, customer.UpdatedAt,
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

func (r *customerRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Customer, error) {
	query := `SELECT id, name, email, phone, COALESCE(password_hash, ''), is_verified, is_guest, created_at, updated_at FROM customers WHERE id = $1`
	return r.scanCustomer(r.db.QueryRow(ctx, query, id))
}

func (r *customerRepository) GetByEmail(ctx context.Context, email string) (*domain.Customer, error) {
	query := `SELECT id, name, email, phone, COALESCE(password_hash, ''), is_verified, is_guest, created_at, updated_at FROM customers WHERE email = $1`
	return r.scanCustomer(r.db.QueryRow(ctx, query, email))
}

func (r *customerRepository) GetByPhone(ctx context.Context, phone string) (*domain.Customer, error) {
	query := `SELECT id, name, email, phone, COALESCE(password_hash, ''), is_verified, is_guest, created_at, updated_at FROM customers WHERE phone = $1`
	return r.scanCustomer(r.db.QueryRow(ctx, query, phone))
}

func (r *customerRepository) scanCustomer(row pgx.Row) (*domain.Customer, error) {
	var c domain.Customer
	err := row.Scan(
		&c.ID, &c.Name, &c.Email, &c.Phone, &c.PasswordHash,
		&c.IsVerified, &c.IsGuest, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *customerRepository) UpdateVerified(ctx context.Context, id uuid.UUID, isVerified bool) error {
	query := `UPDATE customers SET is_verified = $1, updated_at = NOW() WHERE id = $2`
	res, err := r.db.Exec(ctx, query, isVerified, id)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *customerRepository) CreateAddress(ctx context.Context, address *domain.CustomerAddress) error {
	query := `
		INSERT INTO customer_addresses (id, customer_id, label, line1, line2, city, state, country, postal_code, lat, lng, is_default, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	_, err := r.db.Exec(ctx, query,
		address.ID, address.CustomerID, address.Label, address.Line1, address.Line2,
		address.City, address.State, address.Country, address.PostalCode,
		address.Lat, address.Lng, address.IsDefault, address.CreatedAt,
	)
	return err
}

func (r *customerRepository) ListAddresses(ctx context.Context, customerID uuid.UUID) ([]*domain.CustomerAddress, error) {
	query := `
		SELECT id, customer_id, label, line1, line2, city, state, country, postal_code, lat, lng, is_default, created_at
		FROM customer_addresses
		WHERE customer_id = $1
		ORDER BY is_default DESC, created_at DESC
	`
	rows, err := r.db.Query(ctx, query, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var addresses []*domain.CustomerAddress
	for rows.Next() {
		var a domain.CustomerAddress
		if err := rows.Scan(
			&a.ID, &a.CustomerID, &a.Label, &a.Line1, &a.Line2,
			&a.City, &a.State, &a.Country, &a.PostalCode,
			&a.Lat, &a.Lng, &a.IsDefault, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		addresses = append(addresses, &a)
	}
	return addresses, nil
}

// FindByPhone returns the customer with the given phone, or (nil, nil) if none exists.
func (r *customerRepository) FindByPhone(ctx context.Context, phone string) (*domain.Customer, error) {
	query := `
		SELECT id, name, email, phone, COALESCE(password_hash, ''), is_verified, is_guest, created_at, updated_at
		FROM customers
		WHERE phone = $1
	`
	c := &domain.Customer{}
	err := r.db.QueryRow(ctx, query, phone).Scan(
		&c.ID, &c.Name, &c.Email, &c.Phone, &c.PasswordHash,
		&c.IsVerified, &c.IsGuest, &c.CreatedAt, &c.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

// UpdateProfile sets name and email for the customer. An empty email is mapped to
// NULL (via NULLIF) so that the UNIQUE constraint on the nullable email column is
// not violated when multiple customers have no email set.
func (r *customerRepository) UpdateProfile(ctx context.Context, id uuid.UUID, name, email string) error {
	query := `UPDATE customers SET name = $2, email = NULLIF($3, ''), updated_at = NOW() WHERE id = $1`
	res, err := r.db.Exec(ctx, query, id, name, email)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// UpsertByPhone inserts a new customer row with the given phone (name='', is_verified=TRUE)
// if no row with that phone exists; otherwise updates is_verified=TRUE and updated_at=NOW().
// Returns the full customer row.
func (r *customerRepository) UpsertByPhone(ctx context.Context, phone string) (*domain.Customer, error) {
	query := `
		INSERT INTO customers (name, phone, is_verified)
		VALUES ('', $1, TRUE)
		ON CONFLICT (phone) DO UPDATE
		  SET is_verified = TRUE,
		      updated_at  = NOW()
		RETURNING id, name, email, phone, COALESCE(password_hash, ''), is_verified, is_guest, created_at, updated_at
	`
	c := &domain.Customer{}
	err := r.db.QueryRow(ctx, query, phone).Scan(
		&c.ID, &c.Name, &c.Email, &c.Phone, &c.PasswordHash,
		&c.IsVerified, &c.IsGuest, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return c, nil
}
