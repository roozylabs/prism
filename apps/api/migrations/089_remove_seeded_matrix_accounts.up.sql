-- Migration 089: remove the role/plan matrix accounts seeded by 079-081.
-- All 24 share one password that is published in this repository, and the
-- four owners control their organizations. Development databases can
-- recreate them with `go run ./cmd/seed`.

-- Deleting the users cascades to their sessions, accounts, organization and
-- workspace memberships, and anything they own.
DELETE FROM "user" WHERE email IN (
    'owner.free@prism.local', 'dev.free@prism.local', 'agent.free@prism.local',
    'finops.free@prism.local', 'auditor.free@prism.local', 'viewer.free@prism.local',
    'owner.pro@prism.local', 'dev.pro@prism.local', 'agent.pro@prism.local',
    'finops.pro@prism.local', 'auditor.pro@prism.local', 'viewer.pro@prism.local',
    'owner.team@prism.local', 'dev.team@prism.local', 'agent.team@prism.local',
    'finops.team@prism.local', 'auditor.team@prism.local', 'viewer.team@prism.local',
    'owner.enterprise@prism.local', 'dev.enterprise@prism.local', 'agent.enterprise@prism.local',
    'finops.enterprise@prism.local', 'auditor.enterprise@prism.local', 'viewer.enterprise@prism.local'
);

-- 079-081 gave that password to every '%@prism.local' user, which also
-- matches the placeholder emails OAuth sign-in creates. Remove any password
-- login still using it.
DELETE FROM account WHERE password = '$2a$10$rDzx8TEkL6g0kh78Nka8..Citem61KnwQTpbJnJ6xUmGSFPnIYi46';

-- The matrix organizations, with their workspaces, projects and quotas,
-- unless someone else has joined one since.
DELETE FROM organizations o
WHERE o.slug IN ('org-matrix-free', 'org-matrix-pro', 'org-matrix-team', 'org-matrix-enterprise')
  AND NOT EXISTS (SELECT 1 FROM organization_members m WHERE m.org_id = o.id);
