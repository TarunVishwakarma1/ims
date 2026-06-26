-- 000021_address_name_phone.up.sql
-- B2C shop addresses need a contact name + phone so delivery slips render
-- correctly. The B2B flow inferred these from the customer profile.

ALTER TABLE customer_addresses
  ADD COLUMN IF NOT EXISTS name  VARCHAR(120) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS phone VARCHAR(20)  NOT NULL DEFAULT '';
