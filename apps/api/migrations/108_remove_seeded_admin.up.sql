-- Migration 108: remove the seeded admin account.
-- Migration 008 created admin@roozylabs.com (a0eebc99-...) with a password
-- written into the migration itself, and 049 replaced the hash with another
-- one that is also committed, so every fresh database shipped with an
-- account whose credentials are public. System providers were already
-- detached from it (092). Its remaining rows are released or dropped here;
-- the user row then goes and the tables with ON DELETE CASCADE follow.

-- Credentials assigned to the admin by the 085 backfill stay, unowned, so an
-- operator can reassign them instead of losing provider keys.
UPDATE credentials
SET user_id = NULL
WHERE user_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';

-- Seeded routing policies and budget belonged only to this account.
DELETE FROM routing_policies WHERE user_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';
DELETE FROM budgets WHERE user_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';

DELETE FROM session WHERE user_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';
DELETE FROM account WHERE user_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';
DELETE FROM "user" WHERE id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';
