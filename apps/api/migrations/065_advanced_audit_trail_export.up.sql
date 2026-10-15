-- audit_logs is otherwise first created in 074. Create the same baseline
-- table here so a fresh database can index it.
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) DEFAULT 'org_default',
    actor_id VARCHAR(255),
    action VARCHAR(255) NOT NULL,
    resource_type VARCHAR(255) NOT NULL,
    resource_id VARCHAR(255),
    details JSONB DEFAULT '{}'::jsonb,
    ip_address VARCHAR(45),
    user_agent TEXT,
    status VARCHAR(50) DEFAULT 'success',
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create composite indexes for high-performance audit trail queries and compliance exports
CREATE INDEX IF NOT EXISTS idx_audit_logs_org_created ON audit_logs(organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action_status ON audit_logs(action, status);
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor_id ON audit_logs(actor_id);

-- Ensure client connection metadata columns exist
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS actor_ip VARCHAR(45) DEFAULT '';
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS actor_user_agent TEXT DEFAULT '';
