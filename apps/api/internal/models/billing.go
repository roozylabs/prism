package models

import "time"

type BillingInvoice struct {
	ID             string     `json:"id" db:"id"`
	OrganizationID string     `json:"organizationId" db:"org_id"`
	InvoiceNumber  string     `json:"invoiceNumber" db:"invoice_number"`
	AmountDueUSD   float64    `json:"amountDueUsd" db:"amount_due_usd"`
	AmountPaidUSD  float64    `json:"amountPaidUsd" db:"amount_paid_usd"`
	Currency       string     `json:"currency" db:"currency"`
	Status         string     `json:"status" db:"status"` // "paid" | "pending" | "overdue"
	LineItemsJSON  string     `json:"lineItemsJson" db:"line_items_json"`
	PeriodStart    time.Time  `json:"periodStart" db:"period_start"`
	PeriodEnd      time.Time  `json:"periodEnd" db:"period_end"`
	DueDate        time.Time  `json:"dueDate" db:"due_date"`
	PaidAt         *time.Time `json:"paidAt,omitempty" db:"paid_at"`
	CreatedAt      time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt      time.Time  `json:"updatedAt" db:"updated_at"`
}

type DailyUsageAggregate struct {
	ID              string    `json:"id" db:"id"`
	OrganizationID  string    `json:"organizationId" db:"org_id"`
	UsageDate       string    `json:"usageDate" db:"usage_date"`
	ProviderSlug    string    `json:"providerSlug" db:"provider_slug"`
	ModelSlug       string    `json:"modelSlug" db:"model_slug"`
	RequestCount    int       `json:"requestCount" db:"request_count"`
	PromptTokens    int64     `json:"promptTokens" db:"prompt_tokens"`
	CompletionTokens int64    `json:"completionTokens" db:"completion_tokens"`
	ProviderCostUSD float64   `json:"providerCostUsd" db:"provider_cost_usd"`
	MarkupUSD       float64   `json:"markupUsd" db:"markup_usd"`
	CustomerCostUSD float64   `json:"customerCostUsd" db:"customer_cost_usd"`
	CreatedAt       time.Time `json:"createdAt" db:"created_at"`
}

type BillingPlanSummary struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Slug             string   `json:"slug"`
	PriceMonthlyUSD  float64  `json:"priceMonthlyUsd"`
	IncludedTokens   int64    `json:"includedTokens"`
	MarkupPercentage float64  `json:"markupPercentage"`
	Features         []string `json:"features"`
}

type SubscriptionStatusResponse struct {
	Plan              BillingPlanSummary `json:"plan"`
	Status            string             `json:"status"`
	CurrentPeriodEnd  string             `json:"currentPeriodEnd"`
	MonthlyUsageSpent float64            `json:"monthlyUsageSpent"`
}
