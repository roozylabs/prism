package repository

import (
	"context"
	"testing"
)

func TestBillingPlansAndSubscriptionSpendAreOrgScoped(t *testing.T) {
	f := newTenantFixture(t)
	ctx := context.Background()
	r := NewBillingRepository(f.db)

	plans := r.ListPlans()
	if len(plans) != 4 || plans[1].Slug != "pro" || plans[0].PriceMonthlyUSD != 0 {
		t.Fatalf("plans = %+v", plans)
	}

	// This month's spend for org A; org B's and last month's rows must not count.
	mustExec(t, f.db, `INSERT INTO request_logs (request_id, org_id, model, status_code, latency_ms, cost_usd, created_at) VALUES
		(gen_random_uuid(), 'org_alice', 'gpt-4o', 200, 10, 1.25, NOW()),
		(gen_random_uuid(), 'org_alice', 'gpt-4o', 200, 10, 0.75, date_trunc('month', NOW())),
		(gen_random_uuid(), 'org_alice', 'gpt-4o', 200, 10, 9.00, date_trunc('month', NOW()) - interval '1 day'),
		(gen_random_uuid(), 'org_bob',   'gpt-4o', 200, 10, 5.00, NOW())`)

	sub, err := r.GetActiveSubscription(ctx, f.orgA)
	if err != nil {
		t.Fatal(err)
	}
	if sub.Plan.Slug != "pro" || sub.Status != "active" || sub.CurrentPeriodEnd == "" {
		t.Fatalf("subscription = %+v", sub)
	}
	if sub.MonthlyUsageSpent < 1.999 || sub.MonthlyUsageSpent > 2.001 {
		t.Fatalf("org A monthly spend = %v, want 2.00 (this month only)", sub.MonthlyUsageSpent)
	}
}

func TestBillingInvoicesAndDailyUsageAreOrgScoped(t *testing.T) {
	f := newTenantFixture(t)
	ctx := context.Background()
	r := NewBillingRepository(f.db)

	mustExec(t, f.db, `INSERT INTO billing_invoices (org_id, invoice_number, amount_due_usd, amount_paid_usd, status, period_start, period_end, due_date, paid_at, created_at) VALUES
		('org_alice', 'INV-001', 15, 15, 'paid', NOW() - interval '60 days', NOW() - interval '30 days', NOW() - interval '20 days', NOW() - interval '25 days', NOW() - interval '30 days'),
		('org_alice', 'INV-002', 15, 0, 'pending', NOW() - interval '30 days', NOW(), NOW() + interval '10 days', NULL, NOW()),
		('org_bob',   'INV-900', 49, 49, 'paid', NOW() - interval '30 days', NOW(), NOW(), NOW(), NOW())`)

	invoices, err := r.ListInvoices(ctx, f.orgA)
	if err != nil {
		t.Fatal(err)
	}
	if len(invoices) != 2 || invoices[0].InvoiceNumber != "INV-002" || invoices[1].InvoiceNumber != "INV-001" {
		t.Fatalf("invoices = %+v, want INV-002 then INV-001", invoices)
	}
	if invoices[0].PaidAt != nil || invoices[1].PaidAt == nil {
		t.Fatalf("paidAt not mapped: %+v", invoices)
	}
	if invoices[0].LineItemsJSON != "[]" || invoices[0].Currency != "USD" {
		t.Fatalf("defaults not read back: %+v", invoices[0])
	}
	if none, err := r.ListInvoices(ctx, "org_unknown"); err != nil || len(none) != 0 {
		t.Fatalf("unknown org: %v, %v", none, err)
	}

	mustExec(t, f.db, `INSERT INTO daily_usage_aggregates (org_id, usage_date, provider_slug, model_slug, request_count, prompt_tokens, completion_tokens, provider_cost_usd, markup_usd, customer_cost_usd) VALUES
		('org_alice', '2026-10-01', 'openai', 'gpt-4o', 10, 1000, 500, 0.5, 0.05, 0.55),
		('org_alice', '2026-10-02', 'anthropic', 'claude', 3, 300, 100, 0.2, 0.02, 0.22),
		('org_bob',   '2026-10-02', 'openai', 'gpt-4o', 99, 1, 1, 1, 1, 1)`)

	usage, err := r.GetDailyUsage(ctx, f.orgA)
	if err != nil {
		t.Fatal(err)
	}
	if len(usage) != 2 || usage[0].UsageDate != "2026-10-02" || usage[0].ProviderSlug != "anthropic" || usage[1].UsageDate != "2026-10-01" {
		t.Fatalf("usage = %+v, want newest date first", usage)
	}
	if usage[1].RequestCount != 10 || usage[1].PromptTokens != 1000 || usage[1].CustomerCostUSD != 0.55 {
		t.Fatalf("usage row = %+v", usage[1])
	}
}
