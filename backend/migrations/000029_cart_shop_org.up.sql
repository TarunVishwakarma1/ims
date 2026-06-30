-- 000029_cart_shop_org.up.sql
-- P4 phase 3: a cart is bound to a single shop (Zomato-style). Track which
-- seller org the cart belongs to; NULL when the cart is empty. Checkout reads
-- the order's org from here rather than a fixed default org.
ALTER TABLE customer_carts
  ADD COLUMN IF NOT EXISTS org_id UUID REFERENCES organizations(id);
