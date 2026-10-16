package models

import "time"

type TenantQuota struct {
	ID                   string    `json:"id" db:"id"`
	OrganizationID       *string   `json:"organizationId,omitempty" db:"organization_id"`
	WorkspaceID          *string   `json:"workspaceId,omitempty" db:"workspace_id"`
	ProjectID            *string   `json:"projectId,omitempty" db:"project_id"`
	TargetType           string    `json:"targetType" db:"target_type"` // "organization" | "workspace" | "project", derived from the ids above
	TargetID             string    `json:"targetId" db:"target_id"`
	MonthlySpendLimitUSD float64   `json:"monthlySpendLimitUsd" db:"monthly_spend_limit_usd"`
	DailySpendLimitUSD   float64   `json:"dailySpendLimitUsd" db:"daily_spend_limit_usd"`
	DailyRequestLimit    int       `json:"dailyRequestLimit" db:"daily_request_limit"`
	MaxConcurrentStreams int       `json:"maxConcurrentStreams" db:"max_concurrent_streams"`
	CreatedAt            time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt            time.Time `json:"updatedAt" db:"updated_at"`
}

type QuotaCheckResult struct {
	Allowed         bool    `json:"allowed"`
	TargetType      string  `json:"targetType"`
	TargetID        string  `json:"targetId"`
	MonthlySpendUSD float64 `json:"monthlySpendUsd"`
	MonthlyLimitUSD float64 `json:"monthlyLimitUsd"`
	DailySpendUSD   float64 `json:"dailySpendUsd"`
	DailyLimitUSD   float64 `json:"dailyLimitUsd"`
	Reason          string  `json:"reason,omitempty"`
}
