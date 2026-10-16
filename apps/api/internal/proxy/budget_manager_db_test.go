package proxy

import (
	"context"
	"testing"

	"github.com/roozylabs/prism/internal/repository"
)

func TestBudgetStatusChecksEveryBudgetAndTheDailyLimit(t *testing.T) {
	db := migratedTestDB(t)
	// $30 spent today; the monthly limits are far away, the hard budget's
	// daily limit of $20 is not.
	if _, err := db.Exec(`
		INSERT INTO "user" (id, name, email) VALUES ('u1', 'u1', 'u1@example.com');
		INSERT INTO budgets (user_id, name, monthly_limit, daily_limit, hard_limit, created_at)
		VALUES ('u1', 'soft', 1000, 0, false, NOW()),
		       ('u1', 'hard daily', 1000, 20, true, NOW() - interval '1 day');
		WITH k AS (INSERT INTO gateway_api_keys (user_id, name, key_hash, key_prefix) VALUES ('u1', 'k', 'h', 'gw_sk_') RETURNING id)
		INSERT INTO request_logs (model, status_code, latency_ms, cost_usd, gateway_api_key_id) SELECT 'm', 200, 1, 30, id FROM k;`); err != nil {
		t.Fatal(err)
	}
	bm := NewBudgetManager(repository.NewBudgetRepository(db))

	st, err := bm.GetStatus(context.Background(), "u1")
	if err != nil {
		t.Fatal(err)
	}
	// Only the newest budget's monthly limit used to be checked.
	if st == nil || st.Budget.Name != "hard daily" || st.Status != "exceeded" || st.LimitPeriod != "daily" {
		t.Fatalf("status = %+v, want the hard daily budget exceeded", st)
	}
	if st.DailySpent != 30 || st.DailyRemaining != 0 {
		t.Fatalf("daily spend = %v, remaining %v", st.DailySpent, st.DailyRemaining)
	}

	adm := NewAdmissionController(nil, nil, nil, bm)
	res, err := adm.Evaluate(context.Background(), AdmissionRequest{UserID: "u1", ModelSlug: "m"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Decision != AdmissionDeny || res.ErrorCode != "budget_exceeded_error" {
		t.Fatalf("daily hard limit not enforced: %+v", res)
	}

	if st, err := bm.GetStatus(context.Background(), "nobody"); err != nil || st != nil {
		t.Fatalf("user without budgets: %+v, %v", st, err)
	}
}
