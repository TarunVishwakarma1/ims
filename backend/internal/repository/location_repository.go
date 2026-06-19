package repository

import (
	"context"
	"errors"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type LocationRepository interface {
	Create(ctx context.Context, location *domain.OrgLocation) error
	GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*domain.OrgLocation, error)
	List(ctx context.Context, orgID uuid.UUID) ([]*domain.OrgLocation, error)
	Update(ctx context.Context, location *domain.OrgLocation) error
	Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error
	WithTx(tx pgx.Tx) LocationRepository
}

type locationRepository struct {
	db DBTX
}

func NewLocationRepository(db DBTX) LocationRepository {
	return &locationRepository{db: db}
}

func (r *locationRepository) WithTx(tx pgx.Tx) LocationRepository {
	return &locationRepository{db: tx}
}

func (r *locationRepository) Create(ctx context.Context, location *domain.OrgLocation) error {
	query := `
		INSERT INTO org_locations (id, org_id, name, address, city, state, country, postal_code, lat, lng, is_primary, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`
	_, err := r.db.Exec(ctx, query,
		location.ID, location.OrgID, location.Name, location.Address, location.City, location.State,
		location.Country, location.PostalCode, location.Lat, location.Lng, location.IsPrimary, location.IsActive,
		location.CreatedAt, location.UpdatedAt,
	)
	return err
}

func (r *locationRepository) GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*domain.OrgLocation, error) {
	query := `
		SELECT id, org_id, name, address, city, state, country, postal_code, lat, lng, is_primary, is_active, created_at, updated_at
		FROM org_locations
		WHERE id = $1 AND org_id = $2
	`
	var l domain.OrgLocation
	err := r.db.QueryRow(ctx, query, id, orgID).Scan(
		&l.ID, &l.OrgID, &l.Name, &l.Address, &l.City, &l.State, &l.Country,
		&l.PostalCode, &l.Lat, &l.Lng, &l.IsPrimary, &l.IsActive, &l.CreatedAt, &l.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &l, nil
}

func (r *locationRepository) List(ctx context.Context, orgID uuid.UUID) ([]*domain.OrgLocation, error) {
	query := `
		SELECT id, org_id, name, address, city, state, country, postal_code, lat, lng, is_primary, is_active, created_at, updated_at
		FROM org_locations
		WHERE org_id = $1
		ORDER BY name ASC
	`
	rows, err := r.db.Query(ctx, query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var locations []*domain.OrgLocation
	for rows.Next() {
		var l domain.OrgLocation
		if err := rows.Scan(
			&l.ID, &l.OrgID, &l.Name, &l.Address, &l.City, &l.State, &l.Country,
			&l.PostalCode, &l.Lat, &l.Lng, &l.IsPrimary, &l.IsActive, &l.CreatedAt, &l.UpdatedAt,
		); err != nil {
			return nil, err
		}
		locations = append(locations, &l)
	}
	return locations, nil
}

func (r *locationRepository) Update(ctx context.Context, location *domain.OrgLocation) error {
	query := `
		UPDATE org_locations
		SET name = $1, address = $2, city = $3, state = $4, country = $5, postal_code = $6, lat = $7, lng = $8, is_primary = $9, is_active = $10, updated_at = $11
		WHERE id = $12 AND org_id = $13
	`
	res, err := r.db.Exec(ctx, query,
		location.Name, location.Address, location.City, location.State, location.Country, location.PostalCode,
		location.Lat, location.Lng, location.IsPrimary, location.IsActive, location.UpdatedAt,
		location.ID, location.OrgID,
	)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *locationRepository) Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error {
	query := `DELETE FROM org_locations WHERE id = $1 AND org_id = $2`
	res, err := r.db.Exec(ctx, query, id, orgID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
