DO $$
DECLARE r record;
BEGIN
    FOR r IN SELECT indexname FROM pg_indexes
             WHERE schemaname = 'public'
               AND indexname IN ('idx_mcp_registry_org_id', 'idx_member_invites_org_id', 'idx_organization_subscriptions_org_id') LOOP
        EXECUTE format('ALTER INDEX %I RENAME TO %I', r.indexname, replace(r.indexname, 'org_id', 'organization_id'));
    END LOOP;
    FOR r IN SELECT conrelid::regclass AS tbl, conname FROM pg_constraint
             WHERE connamespace = 'public'::regnamespace
               AND conrelid::regclass::text IN ('audit_logs', 'billing_invoices', 'daily_usage_aggregates', 'mcp_registry',
                                                'member_invites', 'organization_subscriptions', 'roles', 'tenant_quotas')
               AND conname LIKE '%org_id%' LOOP
        EXECUTE format('ALTER TABLE %s RENAME CONSTRAINT %I TO %I', r.tbl, r.conname, replace(r.conname, 'org_id', 'organization_id'));
    END LOOP;
END $$;

ALTER TABLE audit_logs                 RENAME COLUMN org_id TO organization_id;
ALTER TABLE billing_invoices           RENAME COLUMN org_id TO organization_id;
ALTER TABLE daily_usage_aggregates     RENAME COLUMN org_id TO organization_id;
ALTER TABLE mcp_registry               RENAME COLUMN org_id TO organization_id;
ALTER TABLE member_invites             RENAME COLUMN org_id TO organization_id;
ALTER TABLE organization_subscriptions RENAME COLUMN org_id TO organization_id;
ALTER TABLE roles                      RENAME COLUMN org_id TO organization_id;
ALTER TABLE tenant_quotas              RENAME COLUMN org_id TO organization_id;

CREATE OR REPLACE VIEW users AS
SELECT id, name, email,
       email_verified AS "emailVerified",
       image,
       created_at AS "createdAt",
       updated_at AS "updatedAt",
       org_id, is_onboarded, primary_role, auth_provider, avatar_url
FROM "user";
