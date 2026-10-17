DROP INDEX IF EXISTS idx_member_invites_pending_email;

ALTER TABLE member_invites
    DROP COLUMN IF EXISTS accepted_at,
    DROP COLUMN IF EXISTS accepted_by,
    DROP COLUMN IF EXISTS invited_by;
