SET search_path TO public;

DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE resource = 'payments'
);
DELETE FROM permissions WHERE resource = 'payments';
