-- 000028_rename_demo_shop.down.sql
UPDATE shop_profiles
   SET slug = 'kirana',
       display_name = 'Kirana',
       updated_at = NOW()
 WHERE slug = 'sharma-kirana';
