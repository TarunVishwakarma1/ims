-- 000030_storefront_permissions.up.sql
-- P4 phase 4: seller storefront self-management. Grants the org admin/manager
-- the ability to view and edit their shop_profile (name, location, go-live).

INSERT INTO permissions (id, resource, action, description) VALUES
    (gen_random_uuid(), 'storefront', 'view',   'View own storefront profile'),
    (gen_random_uuid(), 'storefront', 'manage', 'Edit own storefront profile and go live')
ON CONFLICT (resource, action) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r
JOIN permissions p ON p.resource = 'storefront'
WHERE r.name IN ('admin', 'manager')
ON CONFLICT DO NOTHING;
