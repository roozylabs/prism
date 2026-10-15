-- Migration 092 down: give the system providers back to the seeded admin.
UPDATE providers SET user_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11' WHERE user_id IS NULL;
ALTER TABLE providers ALTER COLUMN user_id SET NOT NULL;
