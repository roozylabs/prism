package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/roozylabs/prism/internal/models"
)

type BillingRepository struct {
	db *sql.DB
}

func NewBillingRepository(db *sql.DB) *BillingRepository {
	return &BillingRepository{db: db}
}

func (r *BillingRepository) ListPlans() []models.BillingPlanSummary {
	return []models.BillingPlanSummary{
		{
			ID:               "plan_free",
			Name:             "Free Tier",
			Slug:             "free",
			PriceMonthlyUSD:  0.0,
			IncludedTokens:   500000,
			MarkupPercentage: 0.0,
			Features:         []string{"Up to $50/mo spend cap", "1 Workspace (2 Projects)", "Standard BYOK Provider Vault (3 Keys)", "Community Discord Support"},
		},
		{
			ID:               "plan_pro",
			Name:             "Pro Developer",
			Slug:             "pro",
			PriceMonthlyUSD:  15.0,
			IncludedTokens:   5000000,
			MarkupPercentage: 0.05,
			Features:         []string{"Up to $300/mo spend cap", "3 Workspaces (5 Projects each)", "All LLM Providers & Adaptive Routing", "Unlimited Vault Credentials", "SHA-256 Cryptographic Audit Logs (30d)"},
		},
		{
			ID:               "plan_team",
			Name:             "Team & Startup",
			Slug:             "team",
			PriceMonthlyUSD:  49.0,
			IncludedTokens:   25000000,
			MarkupPercentage: 0.03,
			Features:         []string{"Up to $1,500/mo spend cap", "10 Workspaces (10 Projects each)", "Paperclip Orchestrator Adapter", "Team RBAC (Up to 15 Members)", "Real-time FinOps Alerts & CSV Export"},
		},
		{
			ID:               "plan_enterprise",
			Name:             "Enterprise Scale",
			Slug:             "enterprise",
			PriceMonthlyUSD:  199.0,
			IncludedTokens:   100000000,
			MarkupPercentage: 0.0,
			Features:         []string{"Up to $5,000+/mo spend cap", "Unlimited Workspaces & Custom Roles", "Dedicated VPC Endpoints & Finetuned Models", "365-day Cold Archive & Merkle Proofs", "99.99% SLA + 24/7 Dedicated TAM"},
		},
	}
}

func (r *BillingRepository) GetActiveSubscription(ctx context.Context, orgID string) (*models.SubscriptionStatusResponse, error) {
	plans := r.ListPlans()

	// Query total monthly spend for specific organization (strict tenant filter)
	querySpent := `
		SELECT COALESCE(SUM(cost_usd), 0.0)
		FROM request_logs
		WHERE created_at >= date_trunc('month', NOW())
		  AND org_id = $1
	`
	var monthlySpent float64
	_ = r.db.QueryRowContext(ctx, querySpent, orgID).Scan(&monthlySpent)

	now := time.Now()
	nextMonth := now.AddDate(0, 1, 0)

	return &models.SubscriptionStatusResponse{
		Plan:              plans[1], // Default Pro plan
		Status:            "active",
		CurrentPeriodEnd:  nextMonth.Format(time.RFC3339),
		MonthlyUsageSpent: monthlySpent,
	}, nil
}

func (r *BillingRepository) ListInvoices(ctx context.Context, orgID string) ([]models.BillingInvoice, error) {
	query := `
		SELECT id, org_id, invoice_number, amount_due_usd, amount_paid_usd,
		       currency, status, line_items_json, period_start, period_end,
		       due_date, paid_at, created_at, updated_at
		FROM billing_invoices
		WHERE org_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return []models.BillingInvoice{}, nil
	}
	defer func() { _ = rows.Close() }()

	var invoices []models.BillingInvoice
	for rows.Next() {
		var inv models.BillingInvoice
		var paidAtNull sql.NullTime
		var lineItemsNull sql.NullString
		if err := rows.Scan(
			&inv.ID, &inv.OrganizationID, &inv.InvoiceNumber, &inv.AmountDueUSD, &inv.AmountPaidUSD,
			&inv.Currency, &inv.Status, &lineItemsNull, &inv.PeriodStart, &inv.PeriodEnd,
			&inv.DueDate, &paidAtNull, &inv.CreatedAt, &inv.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan billing invoice: %w", err)
		}
		if paidAtNull.Valid {
			inv.PaidAt = &paidAtNull.Time
		}
		if lineItemsNull.Valid {
			inv.LineItemsJSON = lineItemsNull.String
		} else {
			inv.LineItemsJSON = "[]"
		}
		invoices = append(invoices, inv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan billing invoices row error: %w", err)
	}

	return invoices, nil
}

func (r *BillingRepository) GetDailyUsage(ctx context.Context, orgID string) ([]models.DailyUsageAggregate, error) {
	query := `
		SELECT id, org_id, usage_date, provider_slug, model_slug,
		       request_count, prompt_tokens, completion_tokens,
		       provider_cost_usd, markup_usd, customer_cost_usd, created_at
		FROM daily_usage_aggregates
		WHERE org_id = $1
		ORDER BY usage_date DESC, provider_slug ASC
	`
	rows, err := r.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return []models.DailyUsageAggregate{}, nil
	}
	defer func() { _ = rows.Close() }()

	var usages []models.DailyUsageAggregate
	for rows.Next() {
		var u models.DailyUsageAggregate
		var usageDate time.Time
		if err := rows.Scan(
			&u.ID, &u.OrganizationID, &usageDate, &u.ProviderSlug, &u.ModelSlug,
			&u.RequestCount, &u.PromptTokens, &u.CompletionTokens,
			&u.ProviderCostUSD, &u.MarkupUSD, &u.CustomerCostUSD, &u.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan daily usage: %w", err)
		}
		u.UsageDate = usageDate.Format("2006-01-02")
		usages = append(usages, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan daily usage row error: %w", err)
	}

	return usages, nil
}
