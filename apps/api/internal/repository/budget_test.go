package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/roozylabs/prism/internal/models"
)

func TestBudgetCRUDIsOwnerScoped(t *testing.T) {
	f := newTenantFixture(t)
	ctx := context.Background()
	r := NewBudgetRepository(f.db)

	first := &models.Budget{UserID: "alice", Name: "ops", MonthlyLimit: 100, DailyLimit: 10, HardLimit: true, WarningThreshold: 0.8, CriticalThreshold: 0.9, Enabled: true}
	second := &models.Budget{UserID: "alice", Name: "research", MonthlyLimit: 50, WarningThreshold: 0.5, CriticalThreshold: 0.9, Enabled: false}
	bobs := &models.Budget{UserID: "bob", Name: "bob", MonthlyLimit: 1, WarningThreshold: 0.8, CriticalThreshold: 0.9, Enabled: true}
	for _, b := range []*models.Budget{first, second, bobs} {
		if err := r.Create(ctx, b); err != nil {
			t.Fatalf("create %s: %v", b.Name, err)
		}
		if b.ID == "" || b.CreatedAt.IsZero() {
			t.Fatalf("create did not fill ids/timestamps: %+v", b)
		}
	}

	list, err := r.ListByUserID(ctx, "alice")
	if err != nil || len(list) != 2 {
		t.Fatalf("alice budgets = %+v, %v", list, err)
	}
	if list[0].Name != "research" || list[1].Name != "ops" {
		t.Fatalf("newest first expected, got %s, %s", list[0].Name, list[1].Name)
	}

	// Migrations seed a budget of their own, so look only at ours.
	enabled, err := r.ListAllEnabled(ctx)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, b := range enabled {
		seen[b.ID] = true
	}
	if seen[second.ID] || !seen[first.ID] || !seen[bobs.ID] {
		t.Fatalf("enabled budgets = %+v, want ops and bob but not research", enabled)
	}

	if _, err := r.FindByID(ctx, first.ID, "bob"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("bob read alice's budget: err = %v", err)
	}
	got, err := r.FindByID(ctx, first.ID, "alice")
	if err != nil || got.MonthlyLimit != 100 || !got.HardLimit {
		t.Fatalf("find = %+v, %v", got, err)
	}

	got.Name = "ops-renamed"
	got.Enabled = false
	if err := r.Update(ctx, got); err != nil {
		t.Fatal(err)
	}
	again, _ := r.FindByID(ctx, first.ID, "alice")
	if again.Name != "ops-renamed" || again.Enabled {
		t.Fatalf("update not persisted: %+v", again)
	}

	if err := r.Delete(ctx, first.ID, "bob"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("bob deleted alice's budget: err = %v", err)
	}
	if err := r.Delete(ctx, first.ID, "alice"); err != nil {
		t.Fatal(err)
	}
	if list, _ := r.ListByUserID(ctx, "alice"); len(list) != 1 {
		t.Fatalf("after delete: %+v", list)
	}
}

func TestBudgetSpendSumsThisMonthAndToday(t *testing.T) {
	f := newTenantFixture(t)
	ctx := context.Background()
	r := NewBudgetRepository(f.db)

	var bobKey string
	if err := f.db.QueryRow(`INSERT INTO gateway_api_keys (user_id, org_id, name, key_hash, key_prefix, enabled)
		VALUES ('bob', 'org_bob', 'bob-key', 'hash-bob', 'prism_sk_bob', true) RETURNING id`).Scan(&bobKey); err != nil {
		t.Fatal(err)
	}
	// Whether the month started today decides if the "earlier this month"
	// row also counts as today's spend.
	var monthStartedToday bool
	if err := f.db.QueryRow(`SELECT date_trunc('month', NOW()) = date_trunc('day', NOW())`).Scan(&monthStartedToday); err != nil {
		t.Fatal(err)
	}
	mustExec(t, f.db, `INSERT INTO request_logs (request_id, gateway_api_key_id, model, status_code, latency_ms, cost_usd, created_at) VALUES
		(gen_random_uuid(), $1, 'gpt-4o', 200, 10, 2.00, NOW()),
		(gen_random_uuid(), $1, 'gpt-4o', 200, 10, 3.00, date_trunc('month', NOW())),
		(gen_random_uuid(), $1, 'gpt-4o', 200, 10, 7.00, date_trunc('month', NOW()) - interval '1 hour'),
		(gen_random_uuid(), $2, 'gpt-4o', 200, 10, 11.00, NOW())`, f.keyAlice, bobKey)

	monthly, daily, err := r.GetCombinedSpend(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	wantDaily := 2.0
	if monthStartedToday {
		wantDaily = 5.0
	}
	if monthly != 5.0 || daily != wantDaily {
		t.Fatalf("combined spend = %v monthly, %v daily; want 5 and %v", monthly, daily, wantDaily)
	}
	if m, err := r.GetTotalMonthlySpend(ctx, "alice"); err != nil || m != 5.0 {
		t.Fatalf("monthly = %v, %v", m, err)
	}
	if d, err := r.GetTotalDailySpend(ctx, "alice"); err != nil || d != wantDaily {
		t.Fatalf("daily = %v, %v", d, err)
	}
	if m, _, err := r.GetCombinedSpend(ctx, "nobody"); err != nil || m != 0 {
		t.Fatalf("unknown user spend = %v, %v", m, err)
	}
}
