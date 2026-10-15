-- 1. Adjust existing "user" table
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS is_onboarded BOOLEAN DEFAULT false;
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS primary_role TEXT DEFAULT 'developer';
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS auth_provider TEXT DEFAULT 'local';
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS avatar_url TEXT;

-- 2. Create permissions catalog table
CREATE TABLE IF NOT EXISTS permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    resource TEXT NOT NULL,
    action TEXT NOT NULL,
    code TEXT UNIQUE NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- 3. Create roles table (system roles & org-custom roles)
CREATE TABLE IF NOT EXISTS roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) REFERENCES organizations(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    description TEXT,
    is_system BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(organization_id, slug)
);

-- 4. Create role_permissions join table
CREATE TABLE IF NOT EXISTS role_permissions (
    role_id UUID REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

-- 5. Adjust organization_members to reference roles(id)
ALTER TABLE organization_members ADD COLUMN IF NOT EXISTS role_id UUID REFERENCES roles(id);

-- 6. Create member_invites table
CREATE TABLE IF NOT EXISTS member_invites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) REFERENCES organizations(id) ON DELETE CASCADE,
    role_id UUID REFERENCES roles(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    token TEXT UNIQUE NOT NULL,
    status TEXT DEFAULT 'pending',
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- 7. Commercial Subscription Plans Table (Prepared for Future Billing Feature)
CREATE TABLE IF NOT EXISTS subscription_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    price_cents_monthly INT NOT NULL DEFAULT 0,
    price_cents_yearly INT NOT NULL DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- 8. Plan Feature Limits Table (Prepared for Future Billing Feature)
CREATE TABLE IF NOT EXISTS plan_features (
    plan_id UUID REFERENCES subscription_plans(id) ON DELETE CASCADE,
    feature_key TEXT NOT NULL,
    feature_value TEXT NOT NULL,
    PRIMARY KEY (plan_id, feature_key)
);

-- 9. Organization Subscriptions Table (Prepared for Future Billing Feature)
CREATE TABLE IF NOT EXISTS organization_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) REFERENCES organizations(id) ON DELETE CASCADE,
    plan_id UUID REFERENCES subscription_plans(id),
    status TEXT DEFAULT 'active',
    current_period_start TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    current_period_end TIMESTAMPTZ DEFAULT (CURRENT_TIMESTAMP + INTERVAL '1 month'),
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- 10. OAuth PKCE Nonce table
CREATE TABLE IF NOT EXISTS oauth_states (
    state TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    redirect_url TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- 11. Seed System Preset Roles & Permissions
INSERT INTO permissions (code, resource, action, description) VALUES
('org:read', 'org', 'read', 'Read Organization details and member list'),
('org:write', 'org', 'write', 'Modify Organization settings and policies'),
('members:invite', 'members', 'invite', 'Invite new team members'),
('members:manage', 'members', 'manage', 'Remove or update member roles'),
('roles:manage', 'roles', 'manage', 'Create and modify custom roles'),
('api_keys:read', 'api_keys', 'read', 'List Gateway API Keys'),
('api_keys:create', 'api_keys', 'create', 'Create new Gateway API Keys'),
('api_keys:revoke', 'api_keys', 'revoke', 'Revoke active Gateway API Keys'),
('credentials:read', 'credentials', 'read', 'List provider credentials'),
('credentials:create', 'credentials', 'create', 'Add provider API keys to Vault'),
('credentials:delete', 'credentials', 'delete', 'Delete provider API credentials'),
('agents:read', 'agents', 'read', 'List autonomous AI Agents'),
('agents:create', 'agents', 'create', 'Create or edit AI Agent identities'),
('agents:manage_budget', 'agents', 'manage_budget', 'Manage AI Agent budget caps'),
('finops:read', 'finops', 'read', 'View FinOps cost & spend reports'),
('finops:manage_budget', 'finops', 'manage_budget', 'Set velocity alert thresholds'),
('billing:manage', 'billing', 'manage', 'Manage billing subscriptions and invoices'),
('audit:read', 'audit', 'read', 'Read AI Audit Logs'),
('audit:export', 'audit', 'export', 'Export audit logs'),
('audit:verify_hash', 'audit', 'verify_hash', 'Verify cryptographic SHA-256 signatures'),
('playground:execute', 'playground', 'execute', 'Execute test prompts in Playground'),
('logs:read', 'logs', 'read', 'View real-time proxy execution logs')
ON CONFLICT (code) DO NOTHING;

INSERT INTO roles (id, organization_id, name, slug, description, is_system) VALUES
('00000000-0000-0000-0000-000000000001', NULL, 'Owner / Org Admin', 'owner', 'Full administrative control over organization and members', true),
('00000000-0000-0000-0000-000000000002', NULL, 'Developer / AI Engineer', 'developer', 'Developer access to API Keys, Playground, Tools, and Logs', true),
('00000000-0000-0000-0000-000000000003', NULL, 'Agent Administrator', 'agent_manager', 'Manage AI Agent identities, model rules, and budgets', true),
('00000000-0000-0000-0000-000000000004', NULL, 'FinOps / Budget Manager', 'finops_manager', 'Manage spend limits, cost analysis, and billing', true),
('00000000-0000-0000-0000-000000000005', NULL, 'Security Auditor', 'auditor', 'Inspect cryptographic audit logs and compliance rules', true),
('00000000-0000-0000-0000-000000000006', NULL, 'Viewer / Guest', 'viewer', 'Read-only dashboard monitoring access', true)
ON CONFLICT (organization_id, slug) DO NOTHING;

-- 12. Seed Default Subscription Plans
INSERT INTO subscription_plans (id, slug, name, description, price_cents_monthly, price_cents_yearly) VALUES
(gen_random_uuid(), 'free', 'Free Tier', 'Ideal for individual developers and small test setups.', 0, 0),
(gen_random_uuid(), 'pro', 'Pro Developer', 'For power developers and growing AI engineering teams.', 2900, 29000),
(gen_random_uuid(), 'team', 'Team & Scale', 'For production AI workloads with team governance.', 14900, 149000),
(gen_random_uuid(), 'enterprise', 'Enterprise Tier', 'For large organizations needing custom SLA & dedicated clusters.', 0, 0)
ON CONFLICT (slug) DO NOTHING;
