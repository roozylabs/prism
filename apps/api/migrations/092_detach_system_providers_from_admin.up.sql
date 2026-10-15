-- Migration 092: system providers have no owner.
-- They belonged to the seeded admin account (a0eebc99-..., admin@roozylabs.com)
-- with ON DELETE CASCADE, so deleting that one user would delete every
-- system provider, all their models, and the credentials and gateway keys
-- every tenant built on them. Providers with no owner are shared with
-- everyone, are changed only by operators, and no user deletion reaches them.
ALTER TABLE providers ALTER COLUMN user_id DROP NOT NULL;
UPDATE providers SET user_id = NULL WHERE user_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';
