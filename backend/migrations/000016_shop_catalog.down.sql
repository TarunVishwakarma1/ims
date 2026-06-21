-- 000016_shop_catalog.down.sql

DROP INDEX IF EXISTS idx_products_shop_catalog;
DROP INDEX IF EXISTS uniq_products_shop_slug;
ALTER TABLE products
  DROP COLUMN IF EXISTS shop_price_paise,
  DROP COLUMN IF EXISTS shop_image_urls,
  DROP COLUMN IF EXISTS shop_description,
  DROP COLUMN IF EXISTS shop_slug,
  DROP COLUMN IF EXISTS shop_visible;

DROP INDEX IF EXISTS idx_categories_shop_visible;
DROP INDEX IF EXISTS uniq_categories_shop_slug;
ALTER TABLE categories
  DROP COLUMN IF EXISTS slug,
  DROP COLUMN IF EXISTS icon_url,
  DROP COLUMN IF EXISTS sort_order,
  DROP COLUMN IF EXISTS shop_visible;
