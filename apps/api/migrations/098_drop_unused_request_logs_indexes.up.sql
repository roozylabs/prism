-- request_logs had 13 indexes, and every insert, one per gateway request,
-- updates all of them. These are not used by any query in the code:
--
--   gateway_api_key_id, agent_id   replaced by the composite indexes of 096/097
--   cost_usd, status_code, model   no query filters or sorts on them alone;
--                                  the log filters always also filter by user
--   (created_at, model),           only ever used for their created_at prefix,
--   (created_at, provider_id)      which idx_request_logs_created_at serves
--
-- Dropping them made inserts about 18% cheaper in a local benchmark and frees
-- the space they took. The down migration recreates them if a plan turns out
-- to need one.
DROP INDEX IF EXISTS idx_request_logs_gateway_api_key_id;
DROP INDEX IF EXISTS idx_request_logs_agent_id;
DROP INDEX IF EXISTS idx_request_logs_cost;
DROP INDEX IF EXISTS idx_request_logs_status_code;
DROP INDEX IF EXISTS idx_request_logs_model;
DROP INDEX IF EXISTS idx_request_logs_date_model;
DROP INDEX IF EXISTS idx_request_logs_date_provider;
