-- 000017_banners.up.sql

CREATE TABLE IF NOT EXISTS banners (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id          UUID NOT NULL REFERENCES organizations(id),
  title           TEXT NOT NULL,
  subtitle        TEXT,
  image_url       TEXT,
  cta_label       TEXT,
  cta_link        TEXT,
  event_key       TEXT,
  starts_at       TIMESTAMPTZ NOT NULL,
  ends_at         TIMESTAMPTZ NOT NULL,
  status          TEXT NOT NULL DEFAULT 'draft'
                  CHECK (status IN ('draft','published','archived')),
  sort_order      INT  NOT NULL DEFAULT 0,
  is_hero         BOOLEAN NOT NULL DEFAULT FALSE,
  audience_filter TEXT,
  category_slug   TEXT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT banner_dates CHECK (ends_at > starts_at)
);

CREATE INDEX IF NOT EXISTS idx_banners_active
  ON banners(org_id, status, starts_at, ends_at)
  WHERE status = 'published';

CREATE UNIQUE INDEX IF NOT EXISTS uniq_banner_hero_active
  ON banners(org_id)
  WHERE is_hero = TRUE AND status = 'published';
