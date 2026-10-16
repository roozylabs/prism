-- organization_members stored a member's role twice: role_id, which
-- permissions are resolved from, and a role string that the owner checks and
-- the member list read. The two drifted apart: an unknown role string fell
-- back to the owner role_id, and changing a member to a role without a roles
-- row only changed the string. Permissions have always followed role_id, so
-- dropping the string changes nobody's access; the slug is read through roles.
ALTER TABLE organization_members DROP CONSTRAINT IF EXISTS organization_members_role_check;
ALTER TABLE organization_members DROP COLUMN IF EXISTS role;
