-- Partial refund support.
-- 1. Track total refunded amount per payment (cumulative across N refunds).
-- 2. Allow `partially_refunded` as a payment status (between captured and refunded).
-- 3. Refund history table so the UI can list every refund + reason + Razorpay id.

SET search_path TO public;

-- 1. amount_refunded column. Default 0 so existing payments don't need a backfill.
ALTER TABLE payments
    ADD COLUMN IF NOT EXISTS amount_refunded BIGINT NOT NULL DEFAULT 0;

-- 2. Loosen status CHECK. Drop and re-add inside a DO block so the migration
--    is idempotent (constraint may already be the wider version on rerun).
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'payments_status_check'
    ) THEN
        ALTER TABLE payments DROP CONSTRAINT payments_status_check;
    END IF;
END$$;

ALTER TABLE payments
    ADD CONSTRAINT payments_status_check CHECK (
        (status)::text = ANY ((ARRAY[
            'created'::character varying,
            'authorized'::character varying,
            'captured'::character varying,
            'failed'::character varying,
            'partially_refunded'::character varying,
            'refunded'::character varying
        ])::text[])
    );

-- Same loosen for orders.payment_status — admin UI flips order to
-- "partial" when payment is partially refunded.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'orders_payment_status_check'
    ) THEN
        ALTER TABLE orders DROP CONSTRAINT orders_payment_status_check;
    END IF;
END$$;

ALTER TABLE orders
    ADD CONSTRAINT orders_payment_status_check CHECK (
        (payment_status)::text = ANY ((ARRAY[
            'unpaid'::character varying,
            'paid'::character varying,
            'partial'::character varying,
            'refunded'::character varying
        ])::text[])
    );

-- 3. Refund history.
CREATE TABLE IF NOT EXISTS refunds (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id           UUID NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    org_id               UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    amount               BIGINT NOT NULL CHECK (amount > 0),
    razorpay_refund_id   TEXT,
    status               VARCHAR(32) NOT NULL DEFAULT 'processed'
                         CHECK (status IN ('processed', 'failed', 'pending')),
    reason               TEXT,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_refunds_payment ON refunds (payment_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_refunds_org     ON refunds (org_id, created_at DESC);
-- Razorpay refund IDs are globally unique; enforce so webhook replays don't
-- double-insert the same refund row.
CREATE UNIQUE INDEX IF NOT EXISTS idx_refunds_rzp_unique
    ON refunds (razorpay_refund_id)
    WHERE razorpay_refund_id IS NOT NULL;
