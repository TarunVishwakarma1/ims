SET search_path TO public;

DROP TABLE IF EXISTS refunds;

ALTER TABLE payments DROP COLUMN IF EXISTS amount_refunded;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'payments_status_check') THEN
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
            'refunded'::character varying
        ])::text[])
    );
