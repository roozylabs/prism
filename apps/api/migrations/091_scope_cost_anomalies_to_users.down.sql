DROP INDEX IF EXISTS idx_cost_anomalies_user_created;
DROP INDEX IF EXISTS uq_cost_anomalies_user_window;
ALTER TABLE cost_anomalies DROP COLUMN IF EXISTS user_id;
