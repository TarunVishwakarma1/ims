-- Sequential invoice numbering per org per Indian financial year.
-- FY runs Apr 1 → Mar 31. Format: INV/<FY>/<SEQ>, e.g. INV/2025-26/00001.

ALTER TABLE orders ADD COLUMN IF NOT EXISTS invoice_number TEXT NULL;

DO $$
BEGIN
    -- Unique across an org. Numbers may collide across orgs (each org gets
    -- its own sequence), so the constraint must be scoped.
    IF NOT EXISTS (
        SELECT 1 FROM pg_indexes
        WHERE indexname = 'orders_org_invoice_unique'
    ) THEN
        CREATE UNIQUE INDEX orders_org_invoice_unique
        ON orders (org_id, invoice_number)
        WHERE invoice_number IS NOT NULL;
    END IF;
END$$;

-- Per-org per-financial-year counter. UPSERT with row-level lock guarantees
-- atomic allocation under concurrency.
CREATE TABLE IF NOT EXISTS org_invoice_sequences (
    org_id     UUID NOT NULL,
    fy_label   TEXT NOT NULL,   -- "2025-26"
    last_seq   BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (org_id, fy_label)
);
