ALTER TABLE request_logs DROP COLUMN IF EXISTS project_id;
ALTER TABLE request_logs DROP COLUMN IF EXISTS workspace_id;
ALTER TABLE request_logs DROP COLUMN IF EXISTS org_id;

ALTER TABLE governance_policies DROP COLUMN IF EXISTS workspace_id;
ALTER TABLE governance_policies DROP COLUMN IF EXISTS org_id;

ALTER TABLE mcp_servers DROP COLUMN IF EXISTS workspace_id;
ALTER TABLE mcp_servers DROP COLUMN IF EXISTS org_id;

ALTER TABLE resources DROP COLUMN IF EXISTS workspace_id;
ALTER TABLE resources DROP COLUMN IF EXISTS org_id;

ALTER TABLE tools DROP COLUMN IF EXISTS workspace_id;
ALTER TABLE tools DROP COLUMN IF EXISTS org_id;

ALTER TABLE agents DROP COLUMN IF EXISTS project_id;
ALTER TABLE agents DROP COLUMN IF EXISTS workspace_id;
ALTER TABLE agents DROP COLUMN IF EXISTS org_id;

ALTER TABLE gateway_api_keys DROP COLUMN IF EXISTS project_id;
ALTER TABLE gateway_api_keys DROP COLUMN IF EXISTS workspace_id;
ALTER TABLE gateway_api_keys DROP COLUMN IF EXISTS org_id;

ALTER TABLE providers DROP COLUMN IF EXISTS workspace_id;
ALTER TABLE providers DROP COLUMN IF EXISTS org_id;

ALTER TABLE "user" DROP COLUMN IF EXISTS org_id;
