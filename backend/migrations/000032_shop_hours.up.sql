-- 000032_shop_hours.up.sql
-- Optional daily business hours for a shop (local IST clock). Both NULL means
-- the shop is always open. closes_at earlier than opens_at wraps past midnight.
ALTER TABLE shop_profiles
  ADD COLUMN IF NOT EXISTS opens_at  TIME,
  ADD COLUMN IF NOT EXISTS closes_at TIME;
