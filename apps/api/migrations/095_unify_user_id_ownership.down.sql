-- Rows deleted because their owner no longer existed are not restored.
CREATE TEMP TABLE parked_policies AS
SELECT tablename, policyname, permissive, roles, cmd, qual, with_check
FROM pg_policies WHERE schemaname = 'public';

DO $$
DECLARE p record;
BEGIN
    FOR p IN SELECT tablename, policyname FROM parked_policies LOOP
        EXECUTE format('DROP POLICY %I ON %I', p.policyname, p.tablename);
    END LOOP;
END $$;

DROP INDEX IF EXISTS idx_agent_templates_user_id;
ALTER TABLE agents              DROP CONSTRAINT IF EXISTS agents_user_id_fkey;
ALTER TABLE tools               DROP CONSTRAINT IF EXISTS tools_user_id_fkey;
ALTER TABLE resources           DROP CONSTRAINT IF EXISTS resources_user_id_fkey;
ALTER TABLE mcp_servers         DROP CONSTRAINT IF EXISTS mcp_servers_user_id_fkey;
ALTER TABLE governance_policies DROP CONSTRAINT IF EXISTS governance_policies_user_id_fkey;
ALTER TABLE mcp_registry        DROP CONSTRAINT IF EXISTS mcp_registry_user_id_fkey;
ALTER TABLE agent_templates     DROP CONSTRAINT IF EXISTS agent_templates_user_id_fkey;

UPDATE agents              SET user_id = 'user_admin' WHERE user_id IS NULL;
UPDATE tools               SET user_id = 'user_admin' WHERE user_id IS NULL;
UPDATE resources           SET user_id = 'user_admin' WHERE user_id IS NULL;
UPDATE mcp_servers         SET user_id = 'user_admin' WHERE user_id IS NULL;
UPDATE governance_policies SET user_id = 'user_admin' WHERE user_id IS NULL;
UPDATE mcp_registry        SET user_id = 'user_admin' WHERE user_id IS NULL;

DROP INDEX IF EXISTS idx_agents_user_name;
DROP INDEX IF EXISTS idx_tools_user_name;
DROP INDEX IF EXISTS idx_resources_user_name;
DROP INDEX IF EXISTS idx_mcp_servers_user_name;
DROP INDEX IF EXISTS idx_governance_policies_user_name;
CREATE UNIQUE INDEX idx_agents_user_name              ON agents (user_id, name);
CREATE UNIQUE INDEX idx_tools_user_name               ON tools (user_id, name);
CREATE UNIQUE INDEX idx_resources_user_name           ON resources (user_id, name);
CREATE UNIQUE INDEX idx_mcp_servers_user_name         ON mcp_servers (user_id, name);
CREATE UNIQUE INDEX idx_governance_policies_user_name ON governance_policies (user_id, name);

ALTER TABLE agents              ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE tools               ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE resources           ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE mcp_servers         ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE governance_policies ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE mcp_registry        ALTER COLUMN user_id SET NOT NULL;

ALTER TABLE agents                 ALTER COLUMN user_id  TYPE varchar(255);
ALTER TABLE tools                  ALTER COLUMN user_id  TYPE varchar(255);
ALTER TABLE resources              ALTER COLUMN user_id  TYPE varchar(255);
ALTER TABLE mcp_servers            ALTER COLUMN user_id  TYPE varchar(255);
ALTER TABLE governance_policies    ALTER COLUMN user_id  TYPE varchar(255);
ALTER TABLE organization_members   ALTER COLUMN user_id  TYPE varchar(64);
ALTER TABLE ai_audit_trails        ALTER COLUMN user_id  TYPE varchar(255);
ALTER TABLE mcp_server_invocations ALTER COLUMN user_id  TYPE varchar(255);
ALTER TABLE audit_logs             ALTER COLUMN actor_id TYPE varchar(255);

DO $$
DECLARE p record;
BEGIN
    FOR p IN SELECT * FROM parked_policies LOOP
        EXECUTE format('CREATE POLICY %I ON %I AS %s FOR %s TO %s%s%s',
            p.policyname, p.tablename, p.permissive, p.cmd,
            (SELECT string_agg(CASE WHEN r = 'public' THEN 'PUBLIC' ELSE quote_ident(r) END, ', ') FROM unnest(p.roles) r),
            CASE WHEN p.qual IS NOT NULL THEN ' USING (' || p.qual || ')' ELSE '' END,
            CASE WHEN p.with_check IS NOT NULL THEN ' WITH CHECK (' || p.with_check || ')' ELSE '' END);
    END LOOP;
END $$;

DROP TABLE parked_policies;
