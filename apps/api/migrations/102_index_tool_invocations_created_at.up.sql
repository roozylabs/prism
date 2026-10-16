-- The retention worker deletes tool_invocations rows older than a cutoff in batches;
-- without an index starting with created_at every batch scans the table.
-- CONCURRENTLY avoids blocking writes; it must be the only statement here.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_tool_invocations_created_at ON tool_invocations (created_at);
