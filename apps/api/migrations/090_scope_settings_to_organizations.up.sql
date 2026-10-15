-- Migration 090: scope settings to organizations.
-- Settings were one global table that any organization owner could rewrite
-- through PUT /api/settings, changing them for every tenant. Rows without an
-- org_id are now platform defaults, which the API no longer writes; each
-- organization stores its own overrides.

ALTER TABLE settings ADD COLUMN IF NOT EXISTS org_id VARCHAR(64) REFERENCES organizations(id) ON DELETE CASCADE;

-- A key is unique per organization, and once among platform defaults.
UPDATE settings SET id = gen_random_uuid()::text WHERE id IS NULL;
ALTER TABLE settings ALTER COLUMN id SET NOT NULL;
ALTER TABLE settings DROP CONSTRAINT IF EXISTS settings_pkey;
ALTER TABLE settings ADD CONSTRAINT settings_pkey PRIMARY KEY (id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_settings_platform_key ON settings (key) WHERE org_id IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_settings_org_key ON settings (org_id, key) WHERE org_id IS NOT NULL;
