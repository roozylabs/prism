-- The retention worker deletes budget_alerts rows older than a cutoff in batches;
-- without an index starting with created_at every batch scans the table.
-- CONCURRENTLY avoids blocking writes; it must be the only statement here.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_budget_alerts_created_at ON budget_alerts (created_at);
