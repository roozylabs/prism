DROP INDEX IF EXISTS idx_audit_logs_actor_id;
DROP INDEX IF EXISTS idx_audit_logs_action_status;
DROP INDEX IF EXISTS idx_audit_logs_org_created;

ALTER TABLE audit_logs DROP COLUMN IF EXISTS actor_user_agent;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS actor_ip;

-- 065 creates audit_logs on a fresh database.
DROP TABLE IF EXISTS audit_logs;
