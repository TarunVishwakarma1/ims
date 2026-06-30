DELETE FROM role_permissions rp
  USING permissions p
 WHERE rp.permission_id = p.id AND p.resource = 'banners';

DELETE FROM permissions WHERE resource = 'banners';
