package featureflags

import (
	"os"
	"strings"
	"sync"
)

type FlagKey string

const (
	FlagSmartRouterAuto         FlagKey = "smart_router_auto"
	FlagPlaygroundMultimodal    FlagKey = "playground_multimodal"
	FlagMCPGateway              FlagKey = "mcp_gateway"
	FlagResourceGateway         FlagKey = "resource_gateway"
	FlagGovernanceGuardrails    FlagKey = "governance_guardrails"
	FlagTurnstileProtection     FlagKey = "turnstile_protection"
	FlagRealtimeAnomalyStream   FlagKey = "realtime_anomaly_stream"
	FlagFinOpsBudgetAlerts      FlagKey = "finops_budget_alerts"
	FlagPaperclipOrchestrator   FlagKey = "paperclip_orchestrator"
	FlagMerkleAuditVerification FlagKey = "merkle_audit_verification"
	FlagCustomRoleBuilder       FlagKey = "custom_role_builder"
	FlagVaultKMSBYOK            FlagKey = "vault_kms_byok"
	FlagBilling                 FlagKey = "billing"
)

type FlagDefinition struct {
	Key          FlagKey  `json:"key"`
	EnvVar       string   `json:"envVar"`
	DefaultState bool     `json:"defaultState"`
	AllowedPlans []string `json:"allowedPlans"`
	Description  string   `json:"description"`
}

var allFlags = []FlagDefinition{
	{
		Key:          FlagSmartRouterAuto,
		EnvVar:       "FEATURE_FLAG_SMART_ROUTER",
		DefaultState: true,
		AllowedPlans: []string{"free", "pro", "team", "enterprise"},
		Description:  "prism-auto dynamic routing & fallback model engine",
	},
	{
		Key:          FlagPlaygroundMultimodal,
		EnvVar:       "FEATURE_FLAG_PLAYGROUND_MULTIMODAL",
		DefaultState: true,
		AllowedPlans: []string{"free", "pro", "team", "enterprise"},
		Description:  "Interactive multimodal vision and audio testing in prompt playground",
	},
	{
		Key:          FlagMCPGateway,
		EnvVar:       "FEATURE_FLAG_MCP_GATEWAY",
		DefaultState: true,
		AllowedPlans: []string{"free", "pro", "team", "enterprise"},
		Description:  "Model Context Protocol servers and tool execution",
	},
	{
		Key:          FlagResourceGateway,
		EnvVar:       "FEATURE_FLAG_RESOURCE_GATEWAY",
		DefaultState: true,
		AllowedPlans: []string{"free", "pro", "team", "enterprise"},
		Description:  "Multi-tenant DB, REST, and GraphQL knowledge context injection",
	},
	{
		Key:          FlagGovernanceGuardrails,
		EnvVar:       "FEATURE_FLAG_GOVERNANCE_GUARDRAILS",
		DefaultState: true,
		AllowedPlans: []string{"free", "pro", "team", "enterprise"},
		Description:  "Prompt guardrails, PII redaction, and keyword safety policies",
	},
	{
		Key:          FlagTurnstileProtection,
		EnvVar:       "FEATURE_FLAG_TURNSTILE_PROTECTION",
		DefaultState: true,
		AllowedPlans: []string{"free", "pro", "team", "enterprise"},
		Description:  "Cloudflare Turnstile anti-bot challenge on authentication surfaces",
	},
	{
		Key:          FlagRealtimeAnomalyStream,
		EnvVar:       "FEATURE_FLAG_REALTIME_ANOMALY",
		DefaultState: false,
		AllowedPlans: []string{"pro", "team", "enterprise"},
		Description:  "Real-time SSE anomaly detection and security telemetry stream",
	},
	{
		Key:          FlagFinOpsBudgetAlerts,
		EnvVar:       "FEATURE_FLAG_FINOPS_BUDGET_ALERTS",
		DefaultState: false,
		AllowedPlans: []string{"team", "enterprise"},
		Description:  "Real-time budget velocity threshold alerting and automated dispatch",
	},
	{
		Key:          FlagPaperclipOrchestrator,
		EnvVar:       "FEATURE_FLAG_PAPERCLIP_ORCHESTRATOR",
		DefaultState: false,
		AllowedPlans: []string{"team", "enterprise"},
		Description:  "Autonomous multi-agent DSL workflow orchestrator",
	},
	{
		Key:          FlagMerkleAuditVerification,
		EnvVar:       "FEATURE_FLAG_MERKLE_AUDIT",
		DefaultState: false,
		AllowedPlans: []string{"enterprise"},
		Description:  "Cryptographic Merkle tree audit log tamper-evident verification",
	},
	{
		Key:          FlagCustomRoleBuilder,
		EnvVar:       "FEATURE_FLAG_CUSTOM_ROLE_BUILDER",
		DefaultState: false,
		AllowedPlans: []string{"enterprise"},
		Description:  "Custom organizational RBAC role creation",
	},
	{
		Key:          FlagVaultKMSBYOK,
		EnvVar:       "FEATURE_FLAG_VAULT_KMS_BYOK",
		DefaultState: false,
		AllowedPlans: []string{"enterprise"},
		Description:  "Hardware AWS KMS and custom key management vault integration",
	},
	{
		// Plans, subscriptions and invoices are placeholders: nothing is
		// stored or charged. Off until billing is real; no plan enables it.
		Key:          FlagBilling,
		EnvVar:       "FEATURE_FLAG_BILLING",
		DefaultState: false,
		Description:  "Plans, subscription and invoice pages",
	},
}

type Manager struct {
	mu    sync.RWMutex
	flags map[FlagKey]FlagDefinition
}

var defaultManager *Manager
var once sync.Once

func GetManager() *Manager {
	once.Do(func() {
		m := &Manager{
			flags: make(map[FlagKey]FlagDefinition),
		}
		for _, def := range allFlags {
			m.flags[def.Key] = def
		}
		defaultManager = m
	})
	return defaultManager
}

// IsEnabled evaluates whether a feature flag is enabled for a given plan tier.
// Evaluation precedence:
// 1. Explicit Environment Variable (e.g. FEATURE_FLAG_PAPERCLIP_ORCHESTRATOR=true/false)
// 2. Plan Tier entitlement check (if plan matches AllowedPlans)
// 3. Flag DefaultState
func (m *Manager) IsEnabled(key FlagKey, planTier string) bool {
	m.mu.RLock()
	def, exists := m.flags[key]
	m.mu.RUnlock()
	if !exists {
		return false
	}

	// 1. Env Variable Override
	if envVal := os.Getenv(def.EnvVar); envVal != "" {
		lower := strings.ToLower(strings.TrimSpace(envVal))
		return lower == "1" || lower == "true" || lower == "yes" || lower == "on"
	}

	// 2. Plan Tier Entitlement
	tier := strings.ToLower(strings.TrimSpace(planTier))
	if tier != "" {
		for _, allowed := range def.AllowedPlans {
			if strings.EqualFold(allowed, tier) {
				return true
			}
		}
	}

	// 3. Fallback to default
	return def.DefaultState
}

// GetAll returns a map of all feature flag states for the provided plan tier.
func (m *Manager) GetAll(planTier string) map[string]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]bool, len(m.flags))
	for key := range m.flags {
		result[string(key)] = m.IsEnabled(key, planTier)
	}
	return result
}
