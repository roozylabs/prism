-- Migration 089 down: intentionally restores nothing. Recreating accounts
-- with a published password is never a useful rollback; use `cmd/seed` on a
-- development database instead.
SELECT 1;
