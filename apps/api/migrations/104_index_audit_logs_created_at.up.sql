-- The retention worker deletes audit_logs rows older than a cutoff in batches;
-- without an index starting with created_at every batch scans the table.
-- CONCURRENTLY avoids blocking writes; it must be the only statement here.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_audit_logs_created_at ON audit_logs (created_at);
