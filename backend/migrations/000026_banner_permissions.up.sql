-- 000026_banner_permissions.up.sql
-- The banner CMS (migration 000017) shipped without its RBAC permissions, so
-- every /api/admin/banners endpoint returned 403 — there was no role that
-- could reach it. Seed the permissions and grant them to the roles that run
-- storefront merchandising.

INSERT INTO permissions (id, resource, action, description) VALUES
    (gen_random_uuid(), 'banners', 'view',   'View home banners'),
    (gen_random_uuid(), 'banners', 'manage', 'Create, schedule, publish and archive home banners')
ON CONFLICT (resource, action) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r
JOIN permissions p ON p.resource = 'banners'
WHERE r.name IN ('admin', 'manager')
ON CONFLICT DO NOTHING;
