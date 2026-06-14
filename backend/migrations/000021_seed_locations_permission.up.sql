INSERT INTO permissions (resource, action, description)
VALUES ('locations', 'manage', 'Manage org locations')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name IN ('admin', 'manager')
AND p.resource = 'locations' AND p.action = 'manage';
