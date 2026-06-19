-- Payment-specific permissions for the new payments admin UI.
-- payments:view  — list payments, view detail, view receipt
-- payments:refund — issue partial or full refunds
-- Admin gets both; manager gets view only by default.

SET search_path TO public;

INSERT INTO permissions (id, resource, action, description) VALUES
    (gen_random_uuid(), 'payments', 'view',   'View payment list, detail, and receipts'),
    (gen_random_uuid(), 'payments', 'refund', 'Issue partial or full refunds')
ON CONFLICT (resource, action) DO NOTHING;

-- Admin: both.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.resource = 'payments'
WHERE r.name = 'admin'
ON CONFLICT DO NOTHING;

-- Manager: view only. Refund must be explicitly granted.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.resource = 'payments' AND p.action = 'view'
WHERE r.name = 'manager'
ON CONFLICT DO NOTHING;
