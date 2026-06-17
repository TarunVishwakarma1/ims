-- New admin-tooling permissions for webhook/notification/audit pages.
-- Previously these items were hacked onto users:delete; carve them out.
-- Admin gets all of them; manager gets the read-only ones; staff gets none.

INSERT INTO permissions (id, resource, action, description) VALUES
    (gen_random_uuid(), 'webhooks',      'view',   'View webhook event history'),
    (gen_random_uuid(), 'webhooks',      'manage', 'Inspect DLQ, replay events, view secrets'),
    (gen_random_uuid(), 'notifications', 'view',   'View notification outbox + DLQ'),
    (gen_random_uuid(), 'notifications', 'manage', 'Resend failed notifications'),
    (gen_random_uuid(), 'audit',         'view',   'Browse the audit log')
ON CONFLICT (resource, action) DO NOTHING;

-- Admin: every new permission.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.resource IN ('webhooks', 'notifications', 'audit')
WHERE r.name = 'admin'
ON CONFLICT DO NOTHING;

-- Manager: read-only views + notifications:manage (operational triage).
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON
       (p.resource = 'webhooks'      AND p.action = 'view')
    OR (p.resource = 'notifications' AND p.action IN ('view', 'manage'))
    OR (p.resource = 'audit'         AND p.action = 'view')
WHERE r.name = 'manager'
ON CONFLICT DO NOTHING;
