-- tenant_quotas named its target with a free-form pair (target_type,
-- target_id). Nothing tied target_id to a row, so a quota outlived the
-- workspace or project it limited, and any string was accepted as a target.
-- The target is now a real foreign key: every quota belongs to an
-- organization, and optionally to one of its workspaces or projects.
-- target_type and target_id stay as generated columns, so readers and the
-- API keep their shape.

ALTER TABLE tenant_quotas
    ADD COLUMN workspace_id VARCHAR(64) REFERENCES workspaces(id) ON DELETE CASCADE,
    ADD COLUMN project_id   VARCHAR(64) REFERENCES projects(id) ON DELETE CASCADE;

-- Resolve existing rows to their target, filling in the owning organization.
UPDATE tenant_quotas q SET organization_id = o.id
FROM organizations o
WHERE q.target_type = 'organization' AND o.id = q.target_id;

UPDATE tenant_quotas q SET workspace_id = w.id, organization_id = w.org_id
FROM workspaces w
WHERE q.target_type = 'workspace' AND w.id = q.target_id;

UPDATE tenant_quotas q SET project_id = p.id, workspace_id = w.id, organization_id = w.org_id
FROM projects p JOIN workspaces w ON w.id = p.workspace_id
WHERE q.target_type = 'project' AND p.id = q.target_id;

-- Quotas whose target does not exist were never enforced: the seeded
-- ('organization', 'default') row, targets deleted since, and the 'agent'
-- and 'user' types, which nothing evaluates.
DELETE FROM tenant_quotas q
WHERE ((q.target_type = 'organization' AND q.organization_id = q.target_id)
    OR (q.target_type = 'workspace' AND q.workspace_id IS NOT NULL)
    OR (q.target_type = 'project' AND q.project_id IS NOT NULL)) IS NOT TRUE;

DROP INDEX IF EXISTS idx_tenant_quotas_target;
ALTER TABLE tenant_quotas
    DROP COLUMN target_type,
    DROP COLUMN target_id,
    ALTER COLUMN organization_id SET NOT NULL,
    ADD CONSTRAINT tenant_quotas_project_has_workspace CHECK (project_id IS NULL OR workspace_id IS NOT NULL),
    ADD COLUMN target_type TEXT GENERATED ALWAYS AS (
        CASE WHEN project_id IS NOT NULL THEN 'project'
             WHEN workspace_id IS NOT NULL THEN 'workspace'
             ELSE 'organization' END) STORED,
    ADD COLUMN target_id VARCHAR(64) GENERATED ALWAYS AS (COALESCE(project_id, workspace_id, organization_id)) STORED;

CREATE UNIQUE INDEX idx_tenant_quotas_target ON tenant_quotas (target_type, target_id);
CREATE INDEX idx_tenant_quotas_workspace_id ON tenant_quotas (workspace_id);
CREATE INDEX idx_tenant_quotas_project_id ON tenant_quotas (project_id);
