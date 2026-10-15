-- Migration 090 down: back to one global row per key. Organization
-- overrides cannot fit that shape and are dropped.
DELETE FROM settings WHERE org_id IS NOT NULL;
DROP INDEX IF EXISTS uq_settings_org_key;
DROP INDEX IF EXISTS uq_settings_platform_key;
ALTER TABLE settings DROP CONSTRAINT IF EXISTS settings_pkey;
ALTER TABLE settings ADD CONSTRAINT settings_pkey PRIMARY KEY (key);
ALTER TABLE settings ALTER COLUMN id DROP NOT NULL;
ALTER TABLE settings DROP COLUMN IF EXISTS org_id;
