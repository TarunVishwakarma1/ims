-- 000014_customer_carts.up.sql
CREATE TABLE IF NOT EXISTS customer_carts (
  customer_id UUID PRIMARY KEY REFERENCES customers(id) ON DELETE CASCADE,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS customer_cart_items (
  customer_id UUID NOT NULL REFERENCES customer_carts(customer_id) ON DELETE CASCADE,
  product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  qty INT NOT NULL CHECK (qty > 0),
  unit_price_paise BIGINT NOT NULL,
  added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (customer_id, product_id)
);

CREATE INDEX IF NOT EXISTS idx_cart_items_product
  ON customer_cart_items(product_id);
