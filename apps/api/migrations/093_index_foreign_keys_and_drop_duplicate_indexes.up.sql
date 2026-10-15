-- Migration 093: index foreign keys, drop duplicate indexes, and make email
-- lookups use an index.
--
-- These are configuration and membership tables, small enough to index
-- without CONCURRENTLY: each build holds its table's write lock only
-- briefly. (CONCURRENTLY also cannot run in a multi-statement migration.)

-- 1. Foreign keys without an index. Deleting an organization, workspace,
-- project, role, plan or user scanned every table below to run its cascade,
-- and tenant filters on these columns could not use an index.
CREATE INDEX IF NOT EXISTS idx_user_org_id ON "user" (org_id);
CREATE INDEX IF NOT EXISTS idx_providers_org_id ON providers (org_id);
CREATE INDEX IF NOT EXISTS idx_providers_workspace_id ON providers (workspace_id);
CREATE INDEX IF NOT EXISTS idx_gateway_api_keys_org_id ON gateway_api_keys (org_id);
CREATE INDEX IF NOT EXISTS idx_gateway_api_keys_workspace_id ON gateway_api_keys (workspace_id);
CREATE INDEX IF NOT EXISTS idx_gateway_api_keys_project_id ON gateway_api_keys (project_id);
CREATE INDEX IF NOT EXISTS idx_routing_rules_provider_id ON routing_rules (provider_id);
CREATE INDEX IF NOT EXISTS idx_budgets_user_id ON budgets (user_id);
CREATE INDEX IF NOT EXISTS idx_tools_org_id ON tools (org_id);
CREATE INDEX IF NOT EXISTS idx_tools_workspace_id ON tools (workspace_id);
CREATE INDEX IF NOT EXISTS idx_resources_org_id ON resources (org_id);
CREATE INDEX IF NOT EXISTS idx_resources_workspace_id ON resources (workspace_id);
CREATE INDEX IF NOT EXISTS idx_mcp_servers_org_id ON mcp_servers (org_id);
CREATE INDEX IF NOT EXISTS idx_mcp_servers_workspace_id ON mcp_servers (workspace_id);
CREATE INDEX IF NOT EXISTS idx_agents_org_id ON agents (org_id);
CREATE INDEX IF NOT EXISTS idx_agents_workspace_id ON agents (workspace_id);
CREATE INDEX IF NOT EXISTS idx_agents_project_id ON agents (project_id);
CREATE INDEX IF NOT EXISTS idx_governance_policies_org_id ON governance_policies (org_id);
CREATE INDEX IF NOT EXISTS idx_governance_policies_workspace_id ON governance_policies (workspace_id);
CREATE INDEX IF NOT EXISTS idx_organization_members_user_id ON organization_members (user_id);
CREATE INDEX IF NOT EXISTS idx_organization_members_role_id ON organization_members (role_id);
CREATE INDEX IF NOT EXISTS idx_mcp_registry_organization_id ON mcp_registry (organization_id);
CREATE INDEX IF NOT EXISTS idx_role_permissions_permission_id ON role_permissions (permission_id);
CREATE INDEX IF NOT EXISTS idx_member_invites_organization_id ON member_invites (organization_id);
CREATE INDEX IF NOT EXISTS idx_member_invites_role_id ON member_invites (role_id);
CREATE INDEX IF NOT EXISTS idx_organization_subscriptions_organization_id ON organization_subscriptions (organization_id);
CREATE INDEX IF NOT EXISTS idx_organization_subscriptions_plan_id ON organization_subscriptions (plan_id);

-- 2. Plain indexes that duplicate a unique constraint's index on the same
-- columns. Every write maintained both. Each is dropped only when the
-- constraint's index exists, in case a database differs from the migrations.
DO $$
DECLARE
    pair TEXT[];
BEGIN
    FOREACH pair SLICE 1 IN ARRAY ARRAY[
        ['idx_session_token', 'session_token_key'],
        ['idx_providers_slug', 'providers_slug_key'],
        ['idx_gateway_api_keys_key_hash', 'gateway_api_keys_key_hash_key'],
        ['idx_org_members_org_user', 'organization_members_org_id_user_id_key'],
        ['idx_mcp_registry_slug', 'mcp_registry_slug_key'],
        ['idx_agent_templates_slug', 'agent_templates_slug_key'],
        ['idx_workspace_members_ws_user', 'workspace_members_workspace_id_user_id_key']
    ] LOOP
        IF to_regclass('public.' || pair[2]) IS NOT NULL THEN
            EXECUTE format('DROP INDEX IF EXISTS %I', pair[1]);
        END IF;
    END LOOP;
END $$;

-- 3. organization_members.role_id had the same foreign key twice.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'organization_members_role_id_fkey'
               AND conrelid = 'organization_members'::regclass) THEN
        ALTER TABLE organization_members DROP CONSTRAINT IF EXISTS fk_org_members_role_id;
    END IF;
END $$;

-- 4. Sign-in looks users up by LOWER(email), which the case-sensitive unique
-- index cannot serve, so every lookup read the whole table. The new index is
-- unique, so 'A@x.com' and 'a@x.com' can no longer be two accounts, unless
-- such pairs already exist: then it is created without uniqueness and the
-- duplicates must be merged by hand before making it unique.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "user" GROUP BY lower(email) HAVING count(*) > 1) THEN
        RAISE WARNING 'user emails that differ only in case exist; created a non-unique index on lower(email)';
        CREATE INDEX IF NOT EXISTS idx_user_email_lower ON "user" (lower(email));
    ELSE
        CREATE UNIQUE INDEX IF NOT EXISTS uq_user_email_lower ON "user" (lower(email));
    END IF;
END $$;
