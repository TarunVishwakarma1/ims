-- 000019_orders_cancelling_status.up.sql

ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_status_check;
ALTER TABLE orders ADD CONSTRAINT orders_status_check
  CHECK (status IN (
    'pending','confirmed','accepted','rejected','processing','ready',
    'shipped','delivered','completed','cancelling','cancelled','refunded'
  ));
