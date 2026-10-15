CREATE TABLE IF NOT EXISTS mcp_registry (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL,
    organization_id VARCHAR(64) REFERENCES organizations(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    slug TEXT UNIQUE NOT NULL,
    description TEXT,
    server_url TEXT NOT NULL,
    transport_type TEXT DEFAULT 'sse',
    visibility TEXT DEFAULT 'private',
    capabilities JSONB DEFAULT '{}'::jsonb,
    is_verified BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_mcp_registry_slug ON mcp_registry(slug);
CREATE INDEX IF NOT EXISTS idx_mcp_registry_visibility ON mcp_registry(visibility);
CREATE INDEX IF NOT EXISTS idx_mcp_registry_user_id ON mcp_registry(user_id);
