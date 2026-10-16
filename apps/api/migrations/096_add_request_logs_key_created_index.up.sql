-- Every per-user spend and analytics query reaches request_logs through the
-- user's gateway keys and a time window: budgets on each request, stats,
-- usage charts and log analytics. (gateway_api_key_id, created_at) serves the
-- key and the window in one range scan, and INCLUDE (cost_usd) lets the
-- budget sums run from the index alone. It replaces the single-column
-- gateway_api_key_id index, which 098 drops, and still covers that foreign
-- key. CONCURRENTLY avoids blocking writes while it builds; it must be the
-- only statement in the file.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_request_logs_key_created
    ON request_logs (gateway_api_key_id, created_at) INCLUDE (cost_usd);
