-- Every resource is owned by one user, "user".id (text). Before this
-- migration the owner column was varchar(255) on some tables and
-- varchar(64) on organization_members, seven resource tables had no foreign
-- key to "user", and system defaults were owned by a made-up user
-- 'user_admin'. Deleting a user left their agents, tools, resources and MCP
-- servers behind.
--
-- After it:
--   * every owner column is text;
--   * system defaults have no owner (user_id IS NULL), as providers do;
--   * resource tables reference "user" ON DELETE CASCADE;
--   * audit and log tables keep the raw id without a foreign key, so their
--     history outlives the user.

-- 1. The row-level security policies read user_id, and Postgres refuses to
-- change the type of a column a policy uses. Park them and restore them
-- unchanged at the end.
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

-- 2. One type for user ids. varchar to text needs no table rewrite.
ALTER TABLE agents                 ALTER COLUMN user_id  TYPE text;
ALTER TABLE tools                  ALTER COLUMN user_id  TYPE text;
ALTER TABLE resources              ALTER COLUMN user_id  TYPE text;
ALTER TABLE mcp_servers            ALTER COLUMN user_id  TYPE text;
ALTER TABLE governance_policies    ALTER COLUMN user_id  TYPE text;
ALTER TABLE organization_members   ALTER COLUMN user_id  TYPE text;
ALTER TABLE ai_audit_trails        ALTER COLUMN user_id  TYPE text;
ALTER TABLE mcp_server_invocations ALTER COLUMN user_id  TYPE text;
ALTER TABLE audit_logs             ALTER COLUMN actor_id TYPE text;

-- 3. System defaults have no owner instead of the pseudo-user 'user_admin'.
ALTER TABLE agents              ALTER COLUMN user_id DROP NOT NULL;
ALTER TABLE tools               ALTER COLUMN user_id DROP NOT NULL;
ALTER TABLE resources           ALTER COLUMN user_id DROP NOT NULL;
ALTER TABLE mcp_servers         ALTER COLUMN user_id DROP NOT NULL;
ALTER TABLE governance_policies ALTER COLUMN user_id DROP NOT NULL;
ALTER TABLE mcp_registry        ALTER COLUMN user_id DROP NOT NULL;

UPDATE agents              SET user_id = NULL WHERE user_id IN ('', 'user_admin');
UPDATE tools               SET user_id = NULL WHERE user_id IN ('', 'user_admin');
UPDATE resources           SET user_id = NULL WHERE user_id IN ('', 'user_admin');
UPDATE mcp_servers         SET user_id = NULL WHERE user_id IN ('', 'user_admin');
UPDATE governance_policies SET user_id = NULL WHERE user_id IN ('', 'user_admin');
UPDATE mcp_registry        SET user_id = NULL WHERE user_id IN ('', 'user_admin');
UPDATE agent_templates     SET user_id = NULL WHERE user_id IN ('', 'user_admin');

-- 4. Rows whose owner was already deleted are unreachable: every read
-- filters by the owner. Remove them as the foreign key would have.
DELETE FROM agents              t WHERE user_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM "user" u WHERE u.id = t.user_id);
DELETE FROM tools               t WHERE user_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM "user" u WHERE u.id = t.user_id);
DELETE FROM resources           t WHERE user_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM "user" u WHERE u.id = t.user_id);
DELETE FROM mcp_servers         t WHERE user_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM "user" u WHERE u.id = t.user_id);
DELETE FROM governance_policies t WHERE user_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM "user" u WHERE u.id = t.user_id);
DELETE FROM mcp_registry        t WHERE user_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM "user" u WHERE u.id = t.user_id);
DELETE FROM agent_templates     t WHERE user_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM "user" u WHERE u.id = t.user_id);

-- 5. Names stay unique among the system defaults too.
DROP INDEX IF EXISTS idx_agents_user_name;
DROP INDEX IF EXISTS idx_tools_user_name;
DROP INDEX IF EXISTS idx_resources_user_name;
DROP INDEX IF EXISTS idx_mcp_servers_user_name;
DROP INDEX IF EXISTS idx_governance_policies_user_name;
CREATE UNIQUE INDEX idx_agents_user_name              ON agents (user_id, name) NULLS NOT DISTINCT;
CREATE UNIQUE INDEX idx_tools_user_name               ON tools (user_id, name) NULLS NOT DISTINCT;
CREATE UNIQUE INDEX idx_resources_user_name           ON resources (user_id, name) NULLS NOT DISTINCT;
CREATE UNIQUE INDEX idx_mcp_servers_user_name         ON mcp_servers (user_id, name) NULLS NOT DISTINCT;
CREATE UNIQUE INDEX idx_governance_policies_user_name ON governance_policies (user_id, name) NULLS NOT DISTINCT;

-- 6. Owned rows go with their owner.
ALTER TABLE agents              ADD CONSTRAINT agents_user_id_fkey              FOREIGN KEY (user_id) REFERENCES "user"(id) ON DELETE CASCADE;
ALTER TABLE tools               ADD CONSTRAINT tools_user_id_fkey               FOREIGN KEY (user_id) REFERENCES "user"(id) ON DELETE CASCADE;
ALTER TABLE resources           ADD CONSTRAINT resources_user_id_fkey           FOREIGN KEY (user_id) REFERENCES "user"(id) ON DELETE CASCADE;
ALTER TABLE mcp_servers         ADD CONSTRAINT mcp_servers_user_id_fkey         FOREIGN KEY (user_id) REFERENCES "user"(id) ON DELETE CASCADE;
ALTER TABLE governance_policies ADD CONSTRAINT governance_policies_user_id_fkey FOREIGN KEY (user_id) REFERENCES "user"(id) ON DELETE CASCADE;
ALTER TABLE mcp_registry        ADD CONSTRAINT mcp_registry_user_id_fkey        FOREIGN KEY (user_id) REFERENCES "user"(id) ON DELETE CASCADE;
ALTER TABLE agent_templates     ADD CONSTRAINT agent_templates_user_id_fkey     FOREIGN KEY (user_id) REFERENCES "user"(id) ON DELETE CASCADE;
CREATE INDEX IF NOT EXISTS idx_agent_templates_user_id ON agent_templates (user_id);

-- 7. Restore the policies exactly as they were.
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
