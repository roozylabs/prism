-- Create Multi-Tenant Quota & Limit Enforcement table
CREATE TABLE IF NOT EXISTS tenant_quotas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) REFERENCES organizations(id) ON DELETE CASCADE,
    target_type VARCHAR(32) NOT NULL, -- 'organization' | 'workspace' | 'agent' | 'user'
    target_id VARCHAR(255) NOT NULL,
    monthly_spend_limit_usd NUMERIC(12, 4) DEFAULT 500.00,
    daily_spend_limit_usd NUMERIC(12, 4) DEFAULT 50.00,
    daily_request_limit INT DEFAULT 10000,
    max_concurrent_streams INT DEFAULT 20,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Unique index to ensure one quota config per target
CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_quotas_target ON tenant_quotas(target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_tenant_quotas_org ON tenant_quotas(organization_id);

-- Seed default organization quota limit
INSERT INTO tenant_quotas (target_type, target_id, monthly_spend_limit_usd, daily_spend_limit_usd, daily_request_limit, max_concurrent_streams)
VALUES ('organization', 'default', 1000.00, 100.00, 50000, 50)
ON CONFLICT (target_type, target_id) DO NOTHING;
