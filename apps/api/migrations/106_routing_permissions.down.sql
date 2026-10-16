DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE code IN ('routing:read', 'routing:update'));
DELETE FROM permissions WHERE code IN ('routing:read', 'routing:update');
