-- Create Multi-Tier Billing Invoices Table
CREATE TABLE IF NOT EXISTS billing_invoices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) REFERENCES organizations(id) ON DELETE CASCADE,
    invoice_number VARCHAR(64) UNIQUE NOT NULL,
    amount_due_usd NUMERIC(12, 4) NOT NULL DEFAULT 0.00,
    amount_paid_usd NUMERIC(12, 4) NOT NULL DEFAULT 0.00,
    currency VARCHAR(10) DEFAULT 'USD',
    status VARCHAR(32) NOT NULL DEFAULT 'paid', -- 'paid' | 'pending' | 'overdue'
    line_items_json JSONB DEFAULT '[]'::jsonb,
    period_start TIMESTAMPTZ NOT NULL,
    period_end TIMESTAMPTZ NOT NULL,
    due_date TIMESTAMPTZ NOT NULL,
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create Daily Usage Aggregates Table for Fast Financial Reporting
CREATE TABLE IF NOT EXISTS daily_usage_aggregates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) REFERENCES organizations(id) ON DELETE CASCADE,
    usage_date DATE NOT NULL,
    provider_slug VARCHAR(64) NOT NULL,
    model_slug VARCHAR(128) NOT NULL,
    request_count INT DEFAULT 0,
    prompt_tokens BIGINT DEFAULT 0,
    completion_tokens BIGINT DEFAULT 0,
    provider_cost_usd NUMERIC(12, 6) DEFAULT 0.000000,
    markup_usd NUMERIC(12, 6) DEFAULT 0.000000,
    customer_cost_usd NUMERIC(12, 6) DEFAULT 0.000000,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unq_daily_usage_org_date_model UNIQUE(organization_id, usage_date, provider_slug, model_slug)
);

CREATE INDEX IF NOT EXISTS idx_billing_invoices_org ON billing_invoices(organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_daily_usage_org_date ON daily_usage_aggregates(organization_id, usage_date DESC);
