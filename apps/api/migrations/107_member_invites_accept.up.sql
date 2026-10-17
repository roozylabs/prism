-- Invitations become acceptable: who sent them, who accepted and when, and one
-- pending invitation per address per organization. Tokens are stored hashed
-- from now on; the plaintext tokens issued before this could never be accepted
-- (there was no endpoint), so they are expired rather than migrated.
ALTER TABLE member_invites
    ADD COLUMN IF NOT EXISTS invited_by TEXT,
    ADD COLUMN IF NOT EXISTS accepted_by TEXT,
    ADD COLUMN IF NOT EXISTS accepted_at TIMESTAMPTZ;

UPDATE member_invites SET status = 'expired' WHERE status = 'pending';

CREATE UNIQUE INDEX IF NOT EXISTS idx_member_invites_pending_email
    ON member_invites (org_id, lower(email))
    WHERE status = 'pending';
