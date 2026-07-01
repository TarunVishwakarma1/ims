package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
)

type shopProfileRepo struct{ db DBTX }

func NewShopProfileRepository(db DBTX) domain.ShopProfileRepository {
	return &shopProfileRepo{db: db}
}

func (r *shopProfileRepo) GetByOrg(ctx context.Context, orgID uuid.UUID) (*domain.ShopProfile, error) {
	var p domain.ShopProfile
	err := r.db.QueryRow(ctx, `
		SELECT org_id, slug, display_name, tagline, logo_url, area, city,
		       pincodes, lat, lng, delivery_radius_km, is_live, created_at, updated_at
		  FROM shop_profiles WHERE org_id = $1`, orgID,
	).Scan(&p.OrgID, &p.Slug, &p.DisplayName, &p.Tagline, &p.LogoURL, &p.Area,
		&p.City, &p.Pincodes, &p.Lat, &p.Lng, &p.DeliveryRadiusKm, &p.IsLive, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *shopProfileRepo) Upsert(ctx context.Context, p *domain.ShopProfile) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO shop_profiles
		    (org_id, slug, display_name, tagline, logo_url, area, city,
		     pincodes, lat, lng, delivery_radius_km, is_live, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12, NOW())
		ON CONFLICT (org_id) DO UPDATE SET
		    slug=$2, display_name=$3, tagline=$4, logo_url=$5, area=$6, city=$7,
		    pincodes=$8, lat=$9, lng=$10, delivery_radius_km=$11, is_live=$12, updated_at=NOW()`,
		p.OrgID, p.Slug, p.DisplayName, p.Tagline, p.LogoURL, p.Area, p.City,
		p.Pincodes, p.Lat, p.Lng, p.DeliveryRadiusKm, p.IsLive)
	return err
}

func (r *shopProfileRepo) SlugTakenByOther(ctx context.Context, slug string, orgID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM shop_profiles WHERE slug=$1 AND org_id<>$2)`,
		slug, orgID,
	).Scan(&exists)
	return exists, err
}
