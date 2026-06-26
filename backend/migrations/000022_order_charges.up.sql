-- 000022_order_charges.up.sql
-- Persist the per-charge breakdown that today is squashed into total_amount.
-- Defaults are 0 so existing rows remain valid. gst_paise duplicates what
-- was previously hidden inside total_amount - subtotal - discount and lets
-- the invoice render a real GST line for legacy orders too.

ALTER TABLE orders
  ADD COLUMN IF NOT EXISTS gst_paise        BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS packing_paise    BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS handling_paise   BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS surge_paise      BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS cod_round_paise  BIGINT NOT NULL DEFAULT 0;
