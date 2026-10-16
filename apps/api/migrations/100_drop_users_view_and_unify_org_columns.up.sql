-- Two leftovers from earlier schema changes.
--
-- 1. The users view was kept for compatibility when the table was renamed to
--    "user" (074/075). Nothing reads it, and because a view freezes its
--    column list when it is created, it no longer shows the user's columns
--    anyway.
DROP VIEW IF EXISTS users;

-- 2. The column that points at organizations(id) was org_id on 14 tables
--    and organization_id on 8. It is org_id everywhere now. Renaming keeps
--    the data, foreign keys, indexes and the expression of the generated
--    tenant_quotas.target_id; the indexes and constraints named after the
--    old column are renamed to match.
ALTER TABLE audit_logs                 RENAME COLUMN organization_id TO org_id;
ALTER TABLE billing_invoices           RENAME COLUMN organization_id TO org_id;
ALTER TABLE daily_usage_aggregates     RENAME COLUMN organization_id TO org_id;
ALTER TABLE mcp_registry               RENAME COLUMN organization_id TO org_id;
ALTER TABLE member_invites             RENAME COLUMN organization_id TO org_id;
ALTER TABLE organization_subscriptions RENAME COLUMN organization_id TO org_id;
ALTER TABLE roles                      RENAME COLUMN organization_id TO org_id;
ALTER TABLE tenant_quotas              RENAME COLUMN organization_id TO org_id;

DO $$
DECLARE r record;
BEGIN
    FOR r IN SELECT conrelid::regclass AS tbl, conname FROM pg_constraint
             WHERE conname LIKE '%organization_id%' AND connamespace = 'public'::regnamespace LOOP
        EXECUTE format('ALTER TABLE %s RENAME CONSTRAINT %I TO %I', r.tbl, r.conname, replace(r.conname, 'organization_id', 'org_id'));
    END LOOP;
    FOR r IN SELECT indexname FROM pg_indexes
             WHERE schemaname = 'public' AND indexname LIKE '%organization_id%' LOOP
        EXECUTE format('ALTER INDEX %I RENAME TO %I', r.indexname, replace(r.indexname, 'organization_id', 'org_id'));
    END LOOP;
END $$;
