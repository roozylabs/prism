package models

import (
	"database/sql"
	"encoding/json"
	"time"
)

const SmartRouterModel = "prism-auto"

type Provider struct {
	ID               string    `json:"id" db:"id"`
	UserID           string    `json:"userId" db:"user_id"`
	Name             string    `json:"name" db:"name"`
	Slug             string    `json:"slug" db:"slug"`
	BaseURL          string    `json:"baseUrl" db:"base_url"`
	Type             string    `json:"type" db:"type"`
	Enabled          bool      `json:"enabled" db:"enabled"`
	RoutingStrategy  string    `json:"routingStrategy" db:"routing_strategy"`
	CreatedAt        time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt        time.Time `json:"updatedAt" db:"updated_at"`
}

type CredentialQuota struct {
	RemainingRequests int64  `json:"remainingRequests,omitempty"`
	LimitRequests     int64  `json:"limitRequests,omitempty"`
	RemainingTokens   int64  `json:"remainingTokens,omitempty"`
	LimitTokens       int64  `json:"limitTokens,omitempty"`
	ResetDurationSec  int    `json:"resetDurationSec,omitempty"`
	ResetAt           string `json:"resetAt,omitempty"`
	StatusText        string `json:"statusText,omitempty"`
	LastUpdated       int64  `json:"lastUpdated,omitempty"`
}

type Credential struct {
	ID                string           `json:"id" db:"id"`
	ProviderID        string           `json:"providerId" db:"provider_id"`
	UserID            *string          `json:"userId,omitempty" db:"user_id"`
	OrgID             *string          `json:"orgId,omitempty" db:"org_id"`
	Name              string           `json:"name" db:"name"`
	EncryptedKey      string           `json:"-" db:"encrypted_key"`
	KeyPrefix         string           `json:"keyPrefix" db:"key_prefix"`
	MaskedKey         string           `json:"maskedKey" db:"masked_key"`
	AuthType          string           `json:"authType" db:"auth_type"`
	EncryptedMetadata sql.NullString   `json:"-" db:"encrypted_metadata"`
	Priority          int              `json:"priority" db:"priority"`
	Enabled           bool             `json:"enabled" db:"enabled"`
	Status            string           `json:"status" db:"status"`
	HealthScore       float64          `json:"healthScore" db:"health_score"`
	LastUsedAt        *time.Time       `json:"lastUsedAt,omitempty" db:"last_used_at"`
	RequestCount      int64            `json:"requestCount" db:"request_count"`
	ErrorCount        int64            `json:"errorCount" db:"error_count"`
	LastError         sql.NullString   `json:"lastError,omitempty" db:"last_error"`
	LastErrorAt       *time.Time       `json:"lastErrorAt,omitempty" db:"last_error_at"`
	CreatedAt         time.Time        `json:"createdAt" db:"created_at"`
	UpdatedAt         time.Time        `json:"updatedAt" db:"updated_at"`
	ProviderName      string           `json:"providerName,omitempty" db:"-"`
	IsCoolingDown     bool             `json:"isCoolingDown,omitempty" db:"-"`
	CooldownTTL       int              `json:"cooldownTtl,omitempty" db:"-"`
	Quota             *CredentialQuota `json:"quota,omitempty" db:"-"`
}

type Model struct {
	ID                string    `json:"id" db:"id"`
	ProviderID        string    `json:"providerId" db:"provider_id"`
	ProviderName      string    `json:"providerName,omitempty" db:"-"`
	Name              string    `json:"name" db:"name"`
	Slug              string    `json:"slug" db:"slug"`
	DisplayName       string    `json:"displayName" db:"display_name"`
	Enabled           bool      `json:"enabled" db:"enabled"`
	ContextWindow     int       `json:"contextWindow" db:"context_window"`
	CodingScore       float64   `json:"codingScore" db:"coding_score"`
	ReasoningScore    float64   `json:"reasoningScore" db:"reasoning_score"`
	WritingScore      float64   `json:"writingScore" db:"writing_score"`
	SpeedScore        float64   `json:"speedScore" db:"speed_score"`
	QualityScore      float64   `json:"qualityScore" db:"quality_score"`
	InputPricePer1M   float64   `json:"inputPricePer1M" db:"input_price_per_1m"`
	OutputPricePer1M  float64   `json:"outputPricePer1M" db:"output_price_per_1m"`
	SupportsTools     bool      `json:"supportsTools" db:"supports_tools"`
	SupportsVision    bool      `json:"supportsVision" db:"supports_vision"`
	CreatedAt         time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt         time.Time `json:"updatedAt" db:"updated_at"`
}

type GatewayAPIKey struct {
	ID            string     `json:"id" db:"id"`
	UserID        string     `json:"userId" db:"user_id"`
	OrgID         *string    `json:"orgId,omitempty" db:"org_id"`
	WorkspaceID   *string    `json:"workspaceId,omitempty" db:"workspace_id"`
	ProjectID     *string    `json:"projectId,omitempty" db:"project_id"`
	ProviderID    *string    `json:"providerId,omitempty" db:"provider_id"`
	Name          string     `json:"name" db:"name"`
	KeyHash       string     `json:"-" db:"key_hash"`
	KeyPrefix     string     `json:"keyPrefix" db:"key_prefix"`
	Enabled       bool       `json:"enabled" db:"enabled"`
	RateLimit     int        `json:"rateLimit" db:"rate_limit"`
	AllowedModels []string   `json:"allowedModels" db:"allowed_models"`
	ExpiresAt     *time.Time `json:"expiresAt,omitempty" db:"expires_at"`
	LastUsedAt    *time.Time `json:"lastUsedAt,omitempty" db:"last_used_at"`
	RequestCount  int64      `json:"requestCount" db:"request_count"`
	CreatedAt     time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt     time.Time  `json:"updatedAt" db:"updated_at"`
}

type RequestLog struct {
	ID              string         `json:"id" db:"id"`
	RequestID       string         `json:"requestId" db:"request_id"`
	OrgID           string         `json:"orgId" db:"org_id"`
	WorkspaceID     string         `json:"workspaceId" db:"workspace_id"`
	ProjectID       string         `json:"projectId" db:"project_id"`
	AgentID         *string        `json:"agentId,omitempty" db:"agent_id"`
	GatewayAPIKeyID *string        `json:"gatewayApiKeyId,omitempty" db:"gateway_api_key_id"`
	ProviderID      *string        `json:"providerId,omitempty" db:"provider_id"`
	ProviderType    string         `json:"providerType,omitempty" db:"-"`
	CredentialID    *string        `json:"credentialId,omitempty" db:"credential_id"`
	CredentialName  string         `json:"credentialName,omitempty" db:"credential_name"`
	Model           string         `json:"model" db:"model"`
	StatusCode      int            `json:"statusCode" db:"status_code"`
	LatencyMs       int            `json:"latencyMs" db:"latency_ms"`
	InputTokens     int            `json:"inputTokens" db:"input_tokens"`
	OutputTokens    int            `json:"outputTokens" db:"output_tokens"`
	TotalTokens     int            `json:"totalTokens" db:"total_tokens"`
	CostUSD         float64        `json:"costUsd" db:"cost_usd"`
	EstimatedCost   float64        `json:"estimatedCost"`
	ErrorMessage    sql.NullString `json:"errorMessage,omitempty" db:"error_message"`
	RetryCount      int            `json:"retryCount" db:"retry_count"`
	ClientIP        string         `json:"clientIp,omitempty" db:"client_ip"`
	UserAgent       string         `json:"userAgent,omitempty" db:"user_agent"`
	ClientApp       string         `json:"clientApp,omitempty" db:"client_app"`
	IsStream        bool           `json:"isStream" db:"is_stream"`
	TTFTMs          int            `json:"ttftMs" db:"ttft_ms"`
	ResponseHash    string         `json:"responseHash" db:"response_hash"`
	ResponseBytes   int            `json:"responseBytes" db:"response_bytes"`
	Attempts        []byte         `json:"attempts,omitempty" db:"attempts"`
	CreatedAt       time.Time      `json:"createdAt" db:"created_at"`
}

// MarshalJSON serializes errorMessage as a plain nullable string instead of the
// default sql.NullString struct shape ({String, Valid}). This keeps the wire
// format consistent with the TypeScript contract `errorMessage?: string`.
func (r RequestLog) MarshalJSON() ([]byte, error) {
	type alias RequestLog
	var errorMessage *string
	if r.ErrorMessage.Valid {
		msg := r.ErrorMessage.String
		errorMessage = &msg
	}
	return json.Marshal(struct {
		alias
		ErrorMessage *string `json:"errorMessage,omitempty"`
	}{
		alias:        alias(r),
		ErrorMessage: errorMessage,
	})
}

type ModelPricing struct {
	ID                     string    `json:"id" db:"id"`
	ModelSlug              string    `json:"modelSlug" db:"model_slug"`
	ProviderType           string    `json:"providerType" db:"provider_type"`
	PromptPricePer1M       float64   `json:"promptPricePer1M" db:"prompt_price_per_1m"`
	CompletionPricePer1M   float64   `json:"completionPricePer1M" db:"completion_price_per_1m"`
	CachedPromptPricePer1M float64   `json:"cachedPromptPricePer1M" db:"cached_prompt_price_per_1m"`
	EffectiveDate          time.Time `json:"effectiveDate" db:"effective_date"`
	CreatedAt              time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt              time.Time `json:"updatedAt" db:"updated_at"`
}

type RoutingPolicy struct {
	ID              string             `json:"id" db:"id"`
	UserID          string             `json:"userId" db:"user_id"`
	Name            string             `json:"name" db:"name"`
	Weights         map[string]float64 `json:"weights" db:"-"`
	Constraints     map[string]float64 `json:"constraints" db:"-"`
	WeightsJSON     string             `json:"-" db:"weights"`
	ConstraintsJSON string             `json:"-" db:"constraints"`
	Enabled         bool               `json:"enabled" db:"enabled"`
	IsDefault       bool               `json:"isDefault" db:"is_default"`
	CreatedAt       time.Time          `json:"createdAt" db:"created_at"`
	UpdatedAt       time.Time          `json:"updatedAt" db:"updated_at"`
}

type Budget struct {
	ID                string    `json:"id" db:"id"`
	UserID            string    `json:"userId" db:"user_id"`
	Name              string    `json:"name" db:"name"`
	MonthlyLimit      float64   `json:"monthlyLimit" db:"monthly_limit"`
	DailyLimit        float64   `json:"dailyLimit" db:"daily_limit"`
	HardLimit         bool      `json:"hardLimit" db:"hard_limit"`
	WarningThreshold  float64   `json:"warningThreshold" db:"warning_threshold"`
	CriticalThreshold float64   `json:"criticalThreshold" db:"critical_threshold"`
	Enabled           bool      `json:"enabled" db:"enabled"`
	CreatedAt         time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt         time.Time `json:"updatedAt" db:"updated_at"`
}

type BudgetStatus struct {
	Budget           *Budget  `json:"budget"`
	MonthlySpent     float64  `json:"monthlySpent"`
	DailySpent       float64  `json:"dailySpent"`
	MonthlyRemaining float64  `json:"monthlyRemaining"`
	DailyRemaining   float64  `json:"dailyRemaining"`
	UsagePercent     float64  `json:"usagePercent"`
	Status           string   `json:"status"`
	// LimitPeriod is the limit UsagePercent and Status refer to: "monthly" or
	// "daily", whichever is closer to being used up.
	LimitPeriod string `json:"limitPeriod,omitempty"`
}

type RoutingRule struct {
	ID           string    `json:"id" db:"id"`
	UserID       string    `json:"userId" db:"user_id"`
	ModelPattern string    `json:"modelPattern" db:"model_pattern"`
	ProviderID   *string   `json:"providerId,omitempty" db:"provider_id"`
	Priority     int       `json:"priority" db:"priority"`
	Enabled      bool      `json:"enabled" db:"enabled"`
	ProviderName string    `json:"providerName,omitempty" db:"-"`
	ProviderType string    `json:"providerType,omitempty" db:"-"`
	CreatedAt    time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt    time.Time `json:"updatedAt" db:"updated_at"`
}

type RequestPayload struct {
	RequestID       string          `json:"requestId" db:"request_id"`
	GatewayAPIKeyID *string         `json:"gatewayApiKeyId,omitempty" db:"gateway_api_key_id"`
	Messages        json.RawMessage `json:"messages" db:"messages"`
	PromptHash      string          `json:"promptHash" db:"prompt_hash"`
	ByteSize        int             `json:"byteSize" db:"byte_size"`
}

type ToolCallRecord struct {
	Name      string          `json:"name"`
	CallID    string          `json:"call_id,omitempty"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

type ToolInvocation struct {
	ID        int64           `db:"id" json:"id"`
	RequestID string          `db:"request_id" json:"request_id"`
	ToolName  string          `db:"tool_name" json:"tool_name"`
	CallID    *string         `db:"call_id" json:"call_id"`
	Arguments json.RawMessage `db:"arguments" json:"arguments"`
	CreatedAt time.Time       `db:"created_at" json:"created_at"`
}
