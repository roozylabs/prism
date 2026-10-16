CREATE INDEX IF NOT EXISTS idx_request_logs_gateway_api_key_id ON request_logs (gateway_api_key_id);
CREATE INDEX IF NOT EXISTS idx_request_logs_agent_id ON request_logs (agent_id);
CREATE INDEX IF NOT EXISTS idx_request_logs_cost ON request_logs (cost_usd);
CREATE INDEX IF NOT EXISTS idx_request_logs_status_code ON request_logs (status_code);
CREATE INDEX IF NOT EXISTS idx_request_logs_model ON request_logs (model);
CREATE INDEX IF NOT EXISTS idx_request_logs_date_model ON request_logs (created_at, model);
CREATE INDEX IF NOT EXISTS idx_request_logs_date_provider ON request_logs (created_at, provider_id);
