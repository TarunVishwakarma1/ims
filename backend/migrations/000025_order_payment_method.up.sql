-- 000025_order_payment_method.up.sql
-- Persist how the customer chose to pay (razorpay | cod) so order history can
-- display the method without inferring it from payment_status.

ALTER TABLE orders
  ADD COLUMN IF NOT EXISTS payment_method TEXT NOT NULL DEFAULT '';
