ALTER TABLE orders DROP COLUMN IF EXISTS order_type;
ALTER TABLE orders DROP COLUMN IF EXISTS buyer_org_id;
ALTER TABLE orders DROP COLUMN IF EXISTS supplier_org_id;
ALTER TABLE orders DROP COLUMN IF EXISTS supplier_location_id;
ALTER TABLE orders DROP COLUMN IF EXISTS customer_id;
ALTER TABLE orders DROP COLUMN IF EXISTS delivery_address_id;
ALTER TABLE orders DROP COLUMN IF EXISTS delivery_address_snapshot;
ALTER TABLE orders DROP COLUMN IF EXISTS subtotal;
ALTER TABLE orders DROP COLUMN IF EXISTS delivery_fee;
ALTER TABLE orders DROP COLUMN IF EXISTS discount;
ALTER TABLE orders DROP COLUMN IF EXISTS payment_status;
ALTER TABLE orders DROP COLUMN IF EXISTS payment_id;
ALTER TABLE orders DROP COLUMN IF EXISTS accepted_at;
ALTER TABLE orders DROP COLUMN IF EXISTS shipped_at;
ALTER TABLE orders DROP COLUMN IF EXISTS delivered_at;
ALTER TABLE orders DROP COLUMN IF EXISTS completed_at;
ALTER TABLE orders DROP COLUMN IF EXISTS cancelled_at;
-- Restore original constraint
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_status_check;
ALTER TABLE orders ADD CONSTRAINT orders_status_check
    CHECK (status IN ('pending', 'confirmed', 'cancelled'));
