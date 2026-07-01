-- 000031_shop_delivery_radius.down.sql
ALTER TABLE shop_profiles DROP COLUMN IF EXISTS delivery_radius_km;
