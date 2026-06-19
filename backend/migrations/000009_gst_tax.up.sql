-- GST tax support.
-- 1. Per-product gst_rate (0..28 inclusive). 0 = exempt.
-- 2. Per-order tax breakdown: total + cgst/sgst (intra-state) or igst
--    (inter-state). Determined by comparing supplier vs buyer primary
--    location state at checkout. is_inter_state caches the decision.

SET search_path TO public;

ALTER TABLE products
    ADD COLUMN IF NOT EXISTS gst_rate INT NOT NULL DEFAULT 0
    CHECK (gst_rate >= 0 AND gst_rate <= 28);

ALTER TABLE orders ADD COLUMN IF NOT EXISTS tax_amount     BIGINT NOT NULL DEFAULT 0;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS tax_cgst       BIGINT NOT NULL DEFAULT 0;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS tax_sgst       BIGINT NOT NULL DEFAULT 0;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS tax_igst       BIGINT NOT NULL DEFAULT 0;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS is_inter_state BOOLEAN NOT NULL DEFAULT FALSE;
