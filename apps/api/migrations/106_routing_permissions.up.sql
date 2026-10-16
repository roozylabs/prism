-- Routing rules, routing policies, routing decisions and the simulator had
-- no permission guard, so every member, a viewer included, could change
-- how traffic is routed. Reading routing stays open to every system role;
-- changing it is for owners and developers.
INSERT INTO permissions (code, resource, action, description) VALUES
('routing:read', 'routing', 'read', 'View routing rules, policies and decisions, and run the routing simulator'),
('routing:update', 'routing', 'update', 'Create, change and delete routing rules and policies')
ON CONFLICT (code) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code = 'routing:read'
WHERE r.is_system AND r.slug IN ('owner', 'developer', 'agent_manager', 'finops_manager', 'auditor', 'viewer')
ON CONFLICT (role_id, permission_id) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code = 'routing:update'
WHERE r.is_system AND r.slug IN ('owner', 'developer')
ON CONFLICT (role_id, permission_id) DO NOTHING;
