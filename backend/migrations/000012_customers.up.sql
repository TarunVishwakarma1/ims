-- Tables `customers` and `customer_addresses` are created in 000001_initial_schema.
-- This migration adds the missing partial unique index required by the B2C shop
-- so that each customer can have at most one default address.

CREATE UNIQUE INDEX IF NOT EXISTS uniq_customer_default_address
  ON customer_addresses(customer_id) WHERE is_default = TRUE;
