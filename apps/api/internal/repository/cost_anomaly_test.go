package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// spendFixture creates users with a gateway key each and lets tests record
// spend at a given time.
type spendFixture struct {
	t    *testing.T
	db   *sql.DB
	keys map[string]string // user id -> gateway key id
}

func newSpendFixture(t *testing.T, db *sql.DB, users ...string) *spendFixture {
	t.Helper()
	f := &spendFixture{t: t, db: db, keys: map[string]string{}}
	for _, u := range users {
		if _, err := db.Exec(`INSERT INTO "user" (id, name, email) VALUES ($1, $1, $1 || '@example.com')`, u); err != nil {
			t.Fatal(err)
		}
		var key string
		if err := db.QueryRow(`INSERT INTO gateway_api_keys (user_id, name, key_hash, key_prefix)
			VALUES ($1, 'k', 'hash-' || $1, 'gw_') RETURNING id`, u).Scan(&key); err != nil {
			t.Fatal(err)
		}
		f.keys[u] = key
	}
	return f
}

func (f *spendFixture) spend(user string, at time.Time, usd float64) {
	f.t.Helper()
	if _, err := f.db.Exec(`INSERT INTO request_logs (gateway_api_key_id, model, status_code, latency_ms, cost_usd, created_at)
		VALUES ($1, 'm', 200, 1, $2, $3)`, f.keys[user], usd, at); err != nil {
		f.t.Fatal(err)
	}
}

func TestCostAnomalySpendIsPerUser(t *testing.T) {
	db := migratedDatabase(t)
	ctx := context.Background()
	f := newSpendFixture(t, db, "alice", "bob")
	r := NewCostAnomalyRepository(db)

	var hour time.Time
	if err := db.QueryRow(`SELECT date_trunc('hour', NOW())`).Scan(&hour); err != nil {
		t.Fatal(err)
	}
	for day := 1; day <= 4; day++ {
		f.spend("alice", hour.AddDate(0, 0, -day).Add(10*time.Minute), float64(day))
		// Another hour of the day must not count toward this hour's baseline.
		f.spend("alice", hour.AddDate(0, 0, -day).Add(-2*time.Hour), 100)
	}
	f.spend("alice", hour.Add(time.Second), 7)
	f.spend("bob", hour.Add(time.Second), 3)

	current, windowStart, err := r.CurrentHourSpend(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !windowStart.Equal(hour) || current["alice"] != 7 || current["bob"] != 3 {
		t.Fatalf("current = %v at %v, want alice 7 and bob 3 at %v", current, windowStart, hour)
	}

	baselines, err := r.SameHourBaselines(ctx, windowStart, 14)
	if err != nil {
		t.Fatal(err)
	}
	if got := baselines["alice"]; len(got) != 4 || sum(got) != 10 {
		t.Fatalf("alice baseline = %v, want the 4 same-hour days summing to 10", got)
	}
	if _, ok := baselines["bob"]; ok {
		t.Fatalf("bob has no history, got %v", baselines["bob"])
	}

	series, err := r.GetDailySpendSeries(ctx, "bob", 28)
	if err != nil {
		t.Fatal(err)
	}
	if len(series) != 1 || series[0].TotalUSD != 3 {
		t.Fatalf("bob's daily series = %v, want only his own spend", series)
	}
}

func TestCostAnomaliesAreListedToTheirOwner(t *testing.T) {
	db := migratedDatabase(t)
	ctx := context.Background()
	newSpendFixture(t, db, "alice", "bob")
	r := NewCostAnomalyRepository(db)

	alice := "alice"
	window := time.Date(2026, 10, 15, 14, 0, 0, 0, time.UTC)
	a := &CostAnomaly{UserID: &alice, WindowStart: window, Metric: "hourly_spend", Observed: 5, Severity: "critical", Details: []byte(`{}`)}
	if inserted, err := r.Create(ctx, a); err != nil || !inserted {
		t.Fatalf("first create: %v, %v", inserted, err)
	}
	if inserted, err := r.Create(ctx, a); err != nil || inserted {
		t.Fatalf("same user, metric and hour again: inserted=%v err=%v, want ignored", inserted, err)
	}
	// A legacy platform-wide row, from before anomalies had owners.
	if _, err := db.Exec(`INSERT INTO cost_anomalies (window_start, metric, observed, severity) VALUES (NOW(), 'hourly_spend', 999, 'critical')`); err != nil {
		t.Fatal(err)
	}

	mine, err := r.ListForUser(ctx, "alice", 50, "")
	if err != nil || len(mine) != 1 || mine[0].Observed != 5 {
		t.Fatalf("alice sees %v, %v; want her one anomaly", mine, err)
	}
	theirs, err := r.ListForUser(ctx, "bob", 50, "")
	if err != nil || len(theirs) != 0 {
		t.Fatalf("bob sees %v, %v; want none", theirs, err)
	}
}

func TestBudgetAlertAcknowledgeChecksOwner(t *testing.T) {
	db := migratedDatabase(t)
	ctx := context.Background()
	newSpendFixture(t, db, "alice", "bob")

	var alertID string
	if err := db.QueryRow(`WITH b AS (INSERT INTO budgets (user_id, name) VALUES ('alice', 'b') RETURNING id)
		INSERT INTO budget_alerts (budget_id, alert_type) SELECT id, 'warning' FROM b RETURNING id`).Scan(&alertID); err != nil {
		t.Fatal(err)
	}
	r := NewBudgetAlertRepository(db)

	if found, err := r.Acknowledge(ctx, alertID, "bob"); err != nil || found {
		t.Fatalf("bob acknowledging alice's alert: found=%v err=%v, want not found", found, err)
	}
	if found, err := r.Acknowledge(ctx, "not-a-uuid", "alice"); err != nil || found {
		t.Fatalf("malformed id: found=%v err=%v, want not found without an error", found, err)
	}
	if found, err := r.Acknowledge(ctx, alertID, "alice"); err != nil || !found {
		t.Fatalf("alice acknowledging her alert: found=%v err=%v", found, err)
	}
}

func sum(vals []float64) float64 {
	var s float64
	for _, v := range vals {
		s += v
	}
	return s
}
