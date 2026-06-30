-- 000027_shop_profiles.up.sql
-- P4 multi-shopkeeper: a consumer-facing storefront profile for an org.
-- Only orgs with a profile (is_live) appear in the shop directory; the
-- customer picks one by serviceable pincode and browses that shop's catalog.

CREATE TABLE IF NOT EXISTS shop_profiles (
    org_id        UUID PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    slug          TEXT NOT NULL UNIQUE,
    display_name  TEXT NOT NULL,
    tagline       TEXT NOT NULL DEFAULT '',
    logo_url      TEXT NOT NULL DEFAULT '',
    area          TEXT NOT NULL DEFAULT '',
    city          TEXT NOT NULL DEFAULT '',
    pincodes      TEXT[] NOT NULL DEFAULT '{}',  -- serviceable delivery pincodes
    lat           DOUBLE PRECISION,
    lng           DOUBLE PRECISION,
    is_live       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Directory lookups filter by serviceable pincode among live shops.
CREATE INDEX IF NOT EXISTS idx_shop_profiles_pincodes ON shop_profiles USING GIN (pincodes);
CREATE INDEX IF NOT EXISTS idx_shop_profiles_live ON shop_profiles (is_live);

-- Seed the existing Kirana org as the first live shop so the directory works
-- out of the box. Slug-based so it's a no-op when the org isn't present.
INSERT INTO shop_profiles (org_id, slug, display_name, tagline, area, city, pincodes, is_live)
SELECT id, 'kirana', name, 'Your neighbourhood kirana — fresh daily', 'Koregaon Park', 'Pune',
       ARRAY['411001', '411036', '411040'], TRUE
  FROM organizations WHERE slug = 'kirana'
ON CONFLICT (org_id) DO NOTHING;
