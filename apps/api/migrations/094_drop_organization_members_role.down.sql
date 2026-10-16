ALTER TABLE organization_members ADD COLUMN IF NOT EXISTS role VARCHAR(32);
UPDATE organization_members om SET role = LEFT(r.slug, 32) FROM roles r WHERE r.id = om.role_id;
ALTER TABLE organization_members ALTER COLUMN role SET NOT NULL;
