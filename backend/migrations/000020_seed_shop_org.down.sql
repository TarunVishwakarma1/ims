-- Reverting drops the Kirana org only if it has no dependent rows; FK
-- ON DELETE CASCADE on most child tables means this WILL wipe banners,
-- categories, products, orders, payments under this org. Operators should
-- prefer disabling SHOP_ENABLED to dropping the org.
DELETE FROM organizations WHERE slug = 'kirana';
