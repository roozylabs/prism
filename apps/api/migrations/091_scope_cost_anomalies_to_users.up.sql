-- Migration 091: detect and list cost anomalies per user.
-- Anomalies were computed over every tenant's spend and listed to anyone
-- signed in. They now belong to one user, the same scope as the rest of
-- FinOps (spend is attributed through the gateway key's owner). Existing rows
-- have no user and are no longer listed.
ALTER TABLE cost_anomalies ADD COLUMN IF NOT EXISTS user_id TEXT REFERENCES "user"(id) ON DELETE CASCADE;

-- The detector runs several times an hour; one anomaly per user, metric and
-- hour is enough.
CREATE UNIQUE INDEX IF NOT EXISTS uq_cost_anomalies_user_window
    ON cost_anomalies (user_id, metric, window_start) WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_cost_anomalies_user_created ON cost_anomalies (user_id, created_at DESC);
