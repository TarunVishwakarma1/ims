DROP TABLE IF EXISTS org_invoice_sequences;
DROP INDEX IF EXISTS orders_org_invoice_unique;
ALTER TABLE orders DROP COLUMN IF EXISTS invoice_number;
