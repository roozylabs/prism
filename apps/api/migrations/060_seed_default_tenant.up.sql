INSERT INTO organizations (id, name, slug, plan_tier) 
VALUES ('org_default', 'Default Organization', 'default-org', 'enterprise')
ON CONFLICT (id) DO NOTHING;

INSERT INTO workspaces (id, org_id, name, slug) 
VALUES ('ws_default', 'org_default', 'Default Workspace', 'default-ws')
ON CONFLICT (id) DO NOTHING;

INSERT INTO projects (id, workspace_id, name, slug) 
VALUES ('proj_default', 'ws_default', 'Default Project', 'default-proj')
ON CONFLICT (id) DO NOTHING;

-- Attach existing single-tenant entities to default tenant boundaries
UPDATE "user" SET org_id = 'org_default' WHERE org_id IS NULL;
UPDATE providers SET org_id = 'org_default', workspace_id = 'ws_default' WHERE org_id IS NULL;
UPDATE gateway_api_keys SET org_id = 'org_default', workspace_id = 'ws_default', project_id = 'proj_default' WHERE org_id IS NULL;
UPDATE agents SET org_id = 'org_default', workspace_id = 'ws_default', project_id = 'proj_default' WHERE org_id IS NULL;
UPDATE tools SET org_id = 'org_default', workspace_id = 'ws_default' WHERE org_id IS NULL;
UPDATE resources SET org_id = 'org_default', workspace_id = 'ws_default' WHERE org_id IS NULL;
UPDATE mcp_servers SET org_id = 'org_default', workspace_id = 'ws_default' WHERE org_id IS NULL;
UPDATE governance_policies SET org_id = 'org_default', workspace_id = 'ws_default' WHERE org_id IS NULL;
UPDATE request_logs SET org_id = 'org_default', workspace_id = 'ws_default', project_id = 'proj_default' WHERE org_id IS NULL;
