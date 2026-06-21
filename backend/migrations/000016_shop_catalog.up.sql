-- 000016_shop_catalog.up.sql

ALTER TABLE categories
  ADD COLUMN IF NOT EXISTS shop_visible BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS sort_order   INT     NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS icon_url     TEXT,
  ADD COLUMN IF NOT EXISTS slug         TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS uniq_categories_shop_slug
  ON categories(org_id, slug) WHERE slug IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_categories_shop_visible
  ON categories(org_id, sort_order) WHERE shop_visible = TRUE;

ALTER TABLE products
  ADD COLUMN IF NOT EXISTS shop_visible      BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS shop_slug         TEXT,
  ADD COLUMN IF NOT EXISTS shop_description  TEXT,
  ADD COLUMN IF NOT EXISTS shop_image_urls   TEXT[],
  ADD COLUMN IF NOT EXISTS shop_price_paise  BIGINT;

CREATE UNIQUE INDEX IF NOT EXISTS uniq_products_shop_slug
  ON products(org_id, shop_slug) WHERE shop_slug IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_products_shop_catalog
  ON products(org_id, category_id) WHERE shop_visible = TRUE;
