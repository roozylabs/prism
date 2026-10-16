-- Quotas deleted because their target did not exist are not restored.
DROP INDEX IF EXISTS idx_tenant_quotas_project_id;
DROP INDEX IF EXISTS idx_tenant_quotas_workspace_id;
DROP INDEX IF EXISTS idx_tenant_quotas_target;

ALTER TABLE tenant_quotas
    ADD COLUMN target_type_old VARCHAR(32),
    ADD COLUMN target_id_old VARCHAR(255);
UPDATE tenant_quotas SET target_type_old = target_type, target_id_old = target_id;

ALTER TABLE tenant_quotas
    DROP COLUMN target_type,
    DROP COLUMN target_id,
    DROP CONSTRAINT IF EXISTS tenant_quotas_project_has_workspace,
    DROP COLUMN project_id,
    DROP COLUMN workspace_id,
    ALTER COLUMN organization_id DROP NOT NULL;
ALTER TABLE tenant_quotas RENAME COLUMN target_type_old TO target_type;
ALTER TABLE tenant_quotas RENAME COLUMN target_id_old TO target_id;
ALTER TABLE tenant_quotas ALTER COLUMN target_type SET NOT NULL, ALTER COLUMN target_id SET NOT NULL;

CREATE UNIQUE INDEX idx_tenant_quotas_target ON tenant_quotas (target_type, target_id);
