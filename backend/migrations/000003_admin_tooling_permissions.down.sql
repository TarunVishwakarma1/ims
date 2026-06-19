DELETE FROM role_permissions
    WHERE permission_id IN (
        SELECT id FROM permissions
        WHERE resource IN ('webhooks', 'notifications', 'audit')
    );

DELETE FROM permissions
    WHERE resource IN ('webhooks', 'notifications', 'audit');
