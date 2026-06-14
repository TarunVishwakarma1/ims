-- Categories
ALTER TABLE categories ADD COLUMN org_id UUID;
UPDATE categories SET org_id = '00000000-0000-0000-0000-000000000000';
ALTER TABLE categories ALTER COLUMN org_id SET NOT NULL;
ALTER TABLE categories ADD CONSTRAINT categories_org_id_fkey FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;

-- Products
ALTER TABLE products ADD COLUMN org_id UUID;
UPDATE products SET org_id = '00000000-0000-0000-0000-000000000000';
ALTER TABLE products ALTER COLUMN org_id SET NOT NULL;
ALTER TABLE products ADD CONSTRAINT products_org_id_fkey FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;

-- Inventory
ALTER TABLE inventory ADD COLUMN org_id UUID;
UPDATE inventory SET org_id = '00000000-0000-0000-0000-000000000000';
ALTER TABLE inventory ALTER COLUMN org_id SET NOT NULL;
ALTER TABLE inventory ADD CONSTRAINT inventory_org_id_fkey FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;

-- Orders
ALTER TABLE orders ADD COLUMN org_id UUID;
UPDATE orders SET org_id = '00000000-0000-0000-0000-000000000000';
ALTER TABLE orders ALTER COLUMN org_id SET NOT NULL;
ALTER TABLE orders ADD CONSTRAINT orders_org_id_fkey FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;

-- Order Items
ALTER TABLE order_items ADD COLUMN org_id UUID;
UPDATE order_items SET org_id = '00000000-0000-0000-0000-000000000000';
ALTER TABLE order_items ALTER COLUMN org_id SET NOT NULL;
ALTER TABLE order_items ADD CONSTRAINT order_items_org_id_fkey FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;

-- Audit Logs
ALTER TABLE audit_logs ADD COLUMN org_id UUID;
UPDATE audit_logs SET org_id = '00000000-0000-0000-0000-000000000000';
ALTER TABLE audit_logs ALTER COLUMN org_id SET NOT NULL;
ALTER TABLE audit_logs ADD CONSTRAINT audit_logs_org_id_fkey FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;
