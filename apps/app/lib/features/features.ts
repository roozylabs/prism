export type FeatureFlagKey =
  | 'smart_router_auto'
  | 'playground_multimodal'
  | 'mcp_gateway'
  | 'resource_gateway'
  | 'governance_guardrails'
  | 'turnstile_protection'
  | 'realtime_anomaly_stream'
  | 'finops_budget_alerts'
  | 'paperclip_orchestrator'
  | 'merkle_audit_verification'
  | 'custom_role_builder'
  | 'vault_kms_byok'
  | 'billing';

export interface ApiFeaturesResponse {
  version: string;
  planTier: string;
  flags: Record<FeatureFlagKey, boolean>;
}

export const DEFAULT_FEATURE_FLAGS: Record<FeatureFlagKey, boolean> = {
  smart_router_auto: true,
  playground_multimodal: true,
  mcp_gateway: true,
  resource_gateway: true,
  governance_guardrails: true,
  turnstile_protection: true,
  realtime_anomaly_stream: false,
  finops_budget_alerts: false,
  paperclip_orchestrator: false,
  merkle_audit_verification: false,
  custom_role_builder: false,
  vault_kms_byok: false,
  // Plans and invoices are placeholders until billing is real (FEATURE_FLAG_BILLING).
  billing: false,
};
