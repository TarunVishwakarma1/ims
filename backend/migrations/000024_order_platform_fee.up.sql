-- 000024_order_platform_fee.up.sql
-- Platform fee is a non-waivable charge that funds infra + support, surfaced
-- as its own invoice line so customers see it itemised.

ALTER TABLE orders
  ADD COLUMN IF NOT EXISTS platform_paise BIGINT NOT NULL DEFAULT 0;
