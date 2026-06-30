-- 000028_rename_demo_shop.up.sql
-- "Kirana" is the marketplace app, not a store. Give the seeded demo tenant
-- its own neighbourhood-store identity so the brand and a seller are distinct.
UPDATE shop_profiles
   SET slug = 'sharma-kirana',
       display_name = 'Sharma Kirana Store',
       updated_at = NOW()
 WHERE slug = 'kirana';
