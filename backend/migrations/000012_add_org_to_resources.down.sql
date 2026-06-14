ALTER TABLE audit_logs DROP CONSTRAINT IF EXISTS audit_logs_org_id_fkey;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS org_id;

ALTER TABLE order_items DROP CONSTRAINT IF EXISTS order_items_org_id_fkey;
ALTER TABLE order_items DROP COLUMN IF EXISTS org_id;

ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_org_id_fkey;
ALTER TABLE orders DROP COLUMN IF EXISTS org_id;

ALTER TABLE inventory DROP CONSTRAINT IF EXISTS inventory_org_id_fkey;
ALTER TABLE inventory DROP COLUMN IF EXISTS org_id;

ALTER TABLE products DROP CONSTRAINT IF EXISTS products_org_id_fkey;
ALTER TABLE products DROP COLUMN IF EXISTS org_id;

ALTER TABLE categories DROP CONSTRAINT IF EXISTS categories_org_id_fkey;
ALTER TABLE categories DROP COLUMN IF EXISTS org_id;
