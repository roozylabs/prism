package workers

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/roozylabs/prism/internal/config"
	"github.com/roozylabs/prism/internal/database"
	"github.com/roozylabs/prism/internal/repository"
)

func migratedDB(t *testing.T) *sql.DB {
	t.Helper()
	base := os.Getenv("DATABASE_URL")
	if base == "" {
		t.Skip("DATABASE_URL is required")
	}
	admin, err := sql.Open("postgres", base)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	if err := admin.Ping(); err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	name := fmt.Sprintf("workers_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE DATABASE " + name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)") })
	u, _ := url.Parse(base)
	u.Path = "/" + name
	if err := database.RunMigrations(u.String(), "../../migrations"); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func count(t *testing.T, db *sql.DB, query string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestRetentionPoliciesNameRealColumns(t *testing.T) {
	db := migratedDB(t)
	all := config.RetentionDays{RequestLogs: 1, RoutingDecisions: 1, RequestPayloads: 1, ToolInvocations: 1,
		MCPInvocations: 1, AIAuditTrails: 1, AuditLogs: 1, CostAnomalies: 1, BudgetAlerts: 1}
	policies := RetentionPolicies(all)
	if len(policies) != 12 {
		t.Fatalf("%d policies, want 9 tables plus 3 expiring ones", len(policies))
	}
	for _, p := range policies {
		if n := count(t, db, fmt.Sprintf(`SELECT count(*) FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = '%s' AND column_name = '%s' AND data_type LIKE 'timestamp%%'`, p.Table, p.Column)); n != 1 {
			t.Errorf("%s.%s is not a timestamp column", p.Table, p.Column)
		}
	}
	// 0 keeps a table forever.
	if got := RetentionPolicies(config.RetentionDays{RequestLogs: 30}); len(got) != 4 || got[0].Table != "request_logs" {
		t.Fatalf("policies with only request_logs set: %+v", got)
	}
}

func TestRetentionDeletesOnlyOldRows(t *testing.T) {
	db := migratedDB(t)
	if _, err := db.Exec(`
		INSERT INTO "user" (id, name, email) VALUES ('u1', 'u1', 'u1@example.com');
		INSERT INTO request_logs (model, status_code, latency_ms, created_at)
		SELECT 'm', 200, 1, NOW() - interval '200 days' FROM generate_series(1, 12)
		UNION ALL SELECT 'm', 200, 1, NOW() - interval '1 day';
		INSERT INTO routing_decisions (user_id, request_id, selected_model, prompt_preview, created_at)
		VALUES ('u1', 'old', 'm', 'secret prompt', NOW() - interval '40 days'), ('u1', 'new', 'm', 'recent prompt', NOW());
		INSERT INTO session (id, token, user_id, expires_at, created_at, updated_at)
		VALUES ('s1', 'expired', 'u1', NOW() - interval '1 hour', NOW(), NOW()),
		       ('s2', 'valid', 'u1', NOW() + interval '1 day', NOW(), NOW());`); err != nil {
		t.Fatal(err)
	}

	purger := repository.NewRetentionRepository(db)
	// Batches of 5 with at most 2 per run: a run stops after 10 rows and the
	// next one finishes the backlog.
	n, err := purger.PurgeBefore(context.Background(), "request_logs", "created_at", time.Now().Add(-180*24*time.Hour), 5, 2, 0)
	if err != nil || n != 10 {
		t.Fatalf("first bounded run deleted %d, %v; want 10", n, err)
	}

	NewRetentionWorker(purger, RetentionPolicies(config.RetentionDays{RequestLogs: 180, RoutingDecisions: 30})).Run(context.Background())

	if got := count(t, db, `SELECT count(*) FROM request_logs`); got != 1 {
		t.Errorf("request_logs left: %d, want only the recent one", got)
	}
	// prompt_preview used to outlive the 30-day payload purge.
	if got := count(t, db, `SELECT count(*) FROM routing_decisions WHERE prompt_preview = 'secret prompt'`); got != 0 {
		t.Error("an old routing decision kept its prompt preview")
	}
	if got := count(t, db, `SELECT count(*) FROM routing_decisions`); got != 1 {
		t.Errorf("routing_decisions left: %d, want 1", got)
	}
	if got := count(t, db, `SELECT count(*) FROM session WHERE token = 'expired'`); got != 0 {
		t.Error("expired session was not deleted")
	}
	if got := count(t, db, `SELECT count(*) FROM session WHERE token = 'valid'`); got != 1 {
		t.Error("valid session was deleted")
	}
}
