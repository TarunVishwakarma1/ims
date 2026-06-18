SET search_path TO public;
ALTER TABLE orders   DROP COLUMN IF EXISTS is_inter_state;
ALTER TABLE orders   DROP COLUMN IF EXISTS tax_igst;
ALTER TABLE orders   DROP COLUMN IF EXISTS tax_sgst;
ALTER TABLE orders   DROP COLUMN IF EXISTS tax_cgst;
ALTER TABLE orders   DROP COLUMN IF EXISTS tax_amount;
ALTER TABLE products DROP COLUMN IF EXISTS gst_rate;
