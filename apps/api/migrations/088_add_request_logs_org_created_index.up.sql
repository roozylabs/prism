-- Serves the per-organization quota spend query (org_id + current month/day,
-- summing cost_usd) as an index-only scan instead of a scan of every
-- request_logs row created this month. CONCURRENTLY avoids blocking writes to
-- request_logs while the index builds; it must be the only statement in the file.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_request_logs_org_created
    ON request_logs (org_id, created_at) INCLUDE (cost_usd);
