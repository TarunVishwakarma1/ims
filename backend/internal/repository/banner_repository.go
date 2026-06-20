package repository

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
)

type BannerRepository interface {
	Insert(ctx context.Context, b *domain.Banner) (*domain.Banner, error)
	Update(ctx context.Context, b *domain.Banner) (*domain.Banner, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (*domain.Banner, error)
	List(ctx context.Context, orgID uuid.UUID, status, eventKey string, limit, offset int) ([]domain.Banner, error)
	ListActive(ctx context.Context, orgID uuid.UUID, categorySlug string, now time.Time) ([]domain.Banner, error)
	ExistsByEventKey(ctx context.Context, orgID uuid.UUID, eventKey string) (bool, error)
	Delete(ctx context.Context, orgID, id uuid.UUID) error
}

type bannerRepo struct{ db DBTX }

func NewBannerRepository(db DBTX) BannerRepository { return &bannerRepo{db} }

func (r *bannerRepo) Insert(ctx context.Context, b *domain.Banner) (*domain.Banner, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO banners (org_id, title, subtitle, image_url, cta_label, cta_link,
			event_key, starts_at, ends_at, status, sort_order, is_hero,
			audience_filter, category_slug)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		RETURNING id, created_at, updated_at
	`,
		b.OrgID, b.Title, nullStr(b.Subtitle), nullStr(b.ImageURL),
		nullStr(b.CTALabel), nullStr(b.CTALink), nullStr(b.EventKey),
		b.StartsAt, b.EndsAt, b.Status, b.SortOrder, b.IsHero,
		nullStr(b.AudienceFilter), nullStr(b.CategorySlug),
	)
	if err := row.Scan(&b.ID, &b.CreatedAt, &b.UpdatedAt); err != nil {
		return nil, err
	}
	return b, nil
}

func (r *bannerRepo) Update(ctx context.Context, b *domain.Banner) (*domain.Banner, error) {
	row := r.db.QueryRow(ctx, `
		UPDATE banners SET title=$3, subtitle=$4, image_url=$5, cta_label=$6, cta_link=$7,
			event_key=$8, starts_at=$9, ends_at=$10, sort_order=$11, is_hero=$12,
			audience_filter=$13, category_slug=$14, updated_at=NOW()
		WHERE id=$1 AND org_id=$2
		RETURNING updated_at
	`,
		b.ID, b.OrgID, b.Title, nullStr(b.Subtitle), nullStr(b.ImageURL),
		nullStr(b.CTALabel), nullStr(b.CTALink), nullStr(b.EventKey),
		b.StartsAt, b.EndsAt, b.SortOrder, b.IsHero,
		nullStr(b.AudienceFilter), nullStr(b.CategorySlug),
	)
	if err := row.Scan(&b.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) { return nil, domain.ErrNotFound }
		return nil, err
	}
	return b, nil
}

func (r *bannerRepo) GetByID(ctx context.Context, orgID, id uuid.UUID) (*domain.Banner, error) {
	var b domain.Banner
	err := r.db.QueryRow(ctx, `
		SELECT id, org_id, title, COALESCE(subtitle,''), COALESCE(image_url,''),
		       COALESCE(cta_label,''), COALESCE(cta_link,''), COALESCE(event_key,''),
		       starts_at, ends_at, status, sort_order, is_hero,
		       COALESCE(audience_filter,''), COALESCE(category_slug,''), created_at, updated_at
		  FROM banners WHERE id=$1 AND org_id=$2
	`, id, orgID).Scan(
		&b.ID, &b.OrgID, &b.Title, &b.Subtitle, &b.ImageURL, &b.CTALabel, &b.CTALink,
		&b.EventKey, &b.StartsAt, &b.EndsAt, &b.Status, &b.SortOrder, &b.IsHero,
		&b.AudienceFilter, &b.CategorySlug, &b.CreatedAt, &b.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) { return nil, domain.ErrNotFound }
	if err != nil { return nil, err }
	return &b, nil
}

func (r *bannerRepo) List(ctx context.Context, orgID uuid.UUID, status, eventKey string, limit, offset int) ([]domain.Banner, error) {
	args := []any{orgID}
	q := `SELECT id, org_id, title, COALESCE(subtitle,''), COALESCE(image_url,''),
	             COALESCE(cta_label,''), COALESCE(cta_link,''), COALESCE(event_key,''),
	             starts_at, ends_at, status, sort_order, is_hero,
	             COALESCE(audience_filter,''), COALESCE(category_slug,''), created_at, updated_at
	        FROM banners WHERE org_id=$1`
	if status != "" {
		args = append(args, status)
		q += " AND status=$" + itoa(len(args))
	}
	if eventKey != "" {
		args = append(args, eventKey)
		q += " AND event_key=$" + itoa(len(args))
	}
	q += " ORDER BY created_at DESC"
	args = append(args, limit, offset)
	q += " LIMIT $" + itoa(len(args)-1) + " OFFSET $" + itoa(len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil { return nil, err }
	defer rows.Close()
	return scanBanners(rows)
}

func (r *bannerRepo) ListActive(ctx context.Context, orgID uuid.UUID, categorySlug string, now time.Time) ([]domain.Banner, error) {
	args := []any{orgID, now}
	q := `SELECT id, org_id, title, COALESCE(subtitle,''), COALESCE(image_url,''),
	             COALESCE(cta_label,''), COALESCE(cta_link,''), COALESCE(event_key,''),
	             starts_at, ends_at, status, sort_order, is_hero,
	             COALESCE(audience_filter,''), COALESCE(category_slug,''), created_at, updated_at
	        FROM banners
	       WHERE org_id=$1 AND status='published'
	         AND $2 BETWEEN starts_at AND ends_at`
	if categorySlug != "" {
		args = append(args, categorySlug)
		q += " AND (category_slug IS NULL OR category_slug=$" + itoa(len(args)) + ")"
	} else {
		q += " AND category_slug IS NULL"
	}
	q += " ORDER BY is_hero DESC, sort_order, created_at DESC"
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil { return nil, err }
	defer rows.Close()
	return scanBanners(rows)
}

func (r *bannerRepo) ExistsByEventKey(ctx context.Context, orgID uuid.UUID, eventKey string) (bool, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT 1 FROM banners WHERE org_id=$1 AND event_key=$2 LIMIT 1`, orgID, eventKey).Scan(&n)
	if errors.Is(err, pgx.ErrNoRows) { return false, nil }
	if err != nil { return false, err }
	return true, nil
}

func (r *bannerRepo) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	ct, err := r.db.Exec(ctx, `DELETE FROM banners WHERE id=$1 AND org_id=$2`, id, orgID)
	if err != nil { return err }
	if ct.RowsAffected() == 0 { return domain.ErrNotFound }
	return nil
}

func scanBanners(rows pgx.Rows) ([]domain.Banner, error) {
	var out []domain.Banner
	for rows.Next() {
		var b domain.Banner
		if err := rows.Scan(
			&b.ID, &b.OrgID, &b.Title, &b.Subtitle, &b.ImageURL, &b.CTALabel, &b.CTALink,
			&b.EventKey, &b.StartsAt, &b.EndsAt, &b.Status, &b.SortOrder, &b.IsHero,
			&b.AudienceFilter, &b.CategorySlug, &b.CreatedAt, &b.UpdatedAt,
		); err != nil { return nil, err }
		out = append(out, b)
	}
	return out, rows.Err()
}

func nullStr(s string) any { if s == "" { return nil }; return s }
func itoa(n int) string    { return strconv.Itoa(n) }
