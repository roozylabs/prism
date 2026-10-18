-- Hashed tokens cannot be turned back into bearer values; every user signs
-- in again.
DELETE FROM session;
