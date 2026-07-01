-- 000031_shop_delivery_radius.up.sql
-- P4 geofencing: an optional delivery radius (km) around the shop's lat/lng.
-- NULL means the shop isn't distance-serviceable and is matched by pincode only.
ALTER TABLE shop_profiles ADD COLUMN IF NOT EXISTS delivery_radius_km DOUBLE PRECISION;
