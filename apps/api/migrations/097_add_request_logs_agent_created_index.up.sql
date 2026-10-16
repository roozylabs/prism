-- Agent stats filter on one agent and a time window. Most requests carry no
-- agent, so the index only holds the rows that do. It replaces the
-- single-column agent_id index, which 098 drops. CONCURRENTLY avoids blocking
-- writes while it builds; it must be the only statement in the file.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_request_logs_agent_created
    ON request_logs (agent_id, created_at) WHERE agent_id IS NOT NULL;
