ALTER TABLE customer_addresses
  DROP COLUMN IF EXISTS name,
  DROP COLUMN IF EXISTS phone;
