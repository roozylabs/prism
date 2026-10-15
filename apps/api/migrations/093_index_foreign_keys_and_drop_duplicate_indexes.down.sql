DROP INDEX IF EXISTS uq_user_email_lower;
DROP INDEX IF EXISTS idx_user_email_lower;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_org_members_role_id') THEN
        ALTER TABLE organization_members ADD CONSTRAINT fk_org_members_role_id FOREIGN KEY (role_id) REFERENCES roles(id);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_session_token ON session (token);
CREATE INDEX IF NOT EXISTS idx_providers_slug ON providers (slug);
CREATE INDEX IF NOT EXISTS idx_gateway_api_keys_key_hash ON gateway_api_keys (key_hash);
CREATE INDEX IF NOT EXISTS idx_org_members_org_user ON organization_members (org_id, user_id);
CREATE INDEX IF NOT EXISTS idx_mcp_registry_slug ON mcp_registry (slug);
CREATE INDEX IF NOT EXISTS idx_agent_templates_slug ON agent_templates (slug);
CREATE INDEX IF NOT EXISTS idx_workspace_members_ws_user ON workspace_members (workspace_id, user_id);

DROP INDEX IF EXISTS idx_organization_subscriptions_plan_id;
DROP INDEX IF EXISTS idx_organization_subscriptions_organization_id;
DROP INDEX IF EXISTS idx_member_invites_role_id;
DROP INDEX IF EXISTS idx_member_invites_organization_id;
DROP INDEX IF EXISTS idx_role_permissions_permission_id;
DROP INDEX IF EXISTS idx_mcp_registry_organization_id;
DROP INDEX IF EXISTS idx_organization_members_role_id;
DROP INDEX IF EXISTS idx_organization_members_user_id;
DROP INDEX IF EXISTS idx_governance_policies_workspace_id;
DROP INDEX IF EXISTS idx_governance_policies_org_id;
DROP INDEX IF EXISTS idx_agents_project_id;
DROP INDEX IF EXISTS idx_agents_workspace_id;
DROP INDEX IF EXISTS idx_agents_org_id;
DROP INDEX IF EXISTS idx_mcp_servers_workspace_id;
DROP INDEX IF EXISTS idx_mcp_servers_org_id;
DROP INDEX IF EXISTS idx_resources_workspace_id;
DROP INDEX IF EXISTS idx_resources_org_id;
DROP INDEX IF EXISTS idx_tools_workspace_id;
DROP INDEX IF EXISTS idx_tools_org_id;
DROP INDEX IF EXISTS idx_budgets_user_id;
DROP INDEX IF EXISTS idx_routing_rules_provider_id;
DROP INDEX IF EXISTS idx_gateway_api_keys_project_id;
DROP INDEX IF EXISTS idx_gateway_api_keys_workspace_id;
DROP INDEX IF EXISTS idx_gateway_api_keys_org_id;
DROP INDEX IF EXISTS idx_providers_workspace_id;
DROP INDEX IF EXISTS idx_providers_org_id;
DROP INDEX IF EXISTS idx_user_org_id;
