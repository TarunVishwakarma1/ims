SET search_path TO public;
DROP TABLE IF EXISTS order_coupons;
DROP TABLE IF EXISTS coupons;
DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE resource = 'coupons'
);
DELETE FROM permissions WHERE resource = 'coupons';
