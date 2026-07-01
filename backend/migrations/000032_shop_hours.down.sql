-- 000032_shop_hours.down.sql
ALTER TABLE shop_profiles
  DROP COLUMN IF EXISTS opens_at,
  DROP COLUMN IF EXISTS closes_at;
