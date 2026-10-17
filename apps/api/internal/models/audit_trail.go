package models

import (
	"time"

	"github.com/lib/pq"
)

type AIAuditTrail struct {
	ID                string         `json:"id"`
	RequestID         string         `json:"requestId"`
	UserID            string         `json:"userId"`
	GatewayKeyID      *string        `json:"gatewayKeyId,omitempty"`
	AgentID           *string        `json:"agentId,omitempty"`
	AgentName         *string        `json:"agentName,omitempty"`
	UserRole          string         `json:"userRole"`
	ModelSlug         string         `json:"modelSlug"`
	FailoverChain     pq.StringArray `json:"failoverChain"`
	ToolsInvoked      pq.StringArray `json:"toolsInvoked"`
	ResourcesAccessed pq.StringArray `json:"resourcesAccessed"`
	MCPServersCalled  pq.StringArray `json:"mcpServersCalled"`
	PromptTokens      int            `json:"promptTokens"`
	CompletionTokens  int            `json:"completionTokens"`
	TotalTokens       int            `json:"totalTokens"`
	TotalCostUSD      float64        `json:"totalCostUsd"`
	StatusCode        int            `json:"statusCode"`
	LatencyMS         int            `json:"latencyMs"`
	TTFTMS            int            `json:"ttftMs"`
	PromptHash        string         `json:"promptHash"`
	ResponseHash      string         `json:"responseHash"`
	ComplianceStatus  string         `json:"complianceStatus"` // "compliant" | "flagged" | "denied"
	SignatureHash     string         `json:"signatureHash"`
	CreatedAt         time.Time      `json:"createdAt"`
}

type AuditTrailFilter struct {
	UserID           string     `json:"userId"`
	AgentName        string     `json:"agentName"`
	ModelSlug        string     `json:"modelSlug"`
	ComplianceStatus string     `json:"complianceStatus"`
	StartDate        *time.Time `json:"startDate"`
	EndDate          *time.Time `json:"endDate"`
	Limit            int        `json:"limit"`
	Offset           int        `json:"offset"`
}

type AuditVerificationResult struct {
	AuditID       string `json:"auditId"`
	RequestID     string `json:"requestId"`
	Valid         bool   `json:"valid"`
	SignatureHash string `json:"signatureHash"`
	ExpectedHash  string `json:"expectedHash"`
	Message       string `json:"message"`
}

type AuditLogItem struct {
	ID             string    `json:"id" db:"id"`
	OrganizationID string    `json:"organizationId" db:"org_id"`
	ActorID        string    `json:"actorId" db:"actor_id"`
	ActorEmail     string    `json:"actorEmail" db:"actor_email"`
	Action         string    `json:"action" db:"action"`
	Resource       string    `json:"resource" db:"resource"`
	ResourceID     string    `json:"resourceId" db:"resource_id"`
	Status         string    `json:"status" db:"status"`
	DetailsJSON    string    `json:"detailsJson" db:"details_json"`
	ActorIP        string    `json:"actorIp" db:"actor_ip"`
	ActorUserAgent string    `json:"actorUserAgent" db:"actor_user_agent"`
	CreatedAt      time.Time `json:"createdAt" db:"created_at"`
}

type AuditExportRequest struct {
	Format    string `json:"format"` // "csv" | "json"
	Action    string `json:"action"`
	Status    string `json:"status"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	Search    string `json:"search"`
	// Since and Until are StartDate and EndDate parsed by the handler; the
	// repository filters on them.
	Since *time.Time `json:"-"`
	Until *time.Time `json:"-"`
}
