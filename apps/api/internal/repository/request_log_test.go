package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/roozylabs/prism/internal/models"
)

// requestLogFixture adds bob's key and a credential on alice's provider,
// then writes four logs through the repository: three for alice (one
// failed, one by an agent, one streamed) and one for bob.
type requestLogFixture struct {
	*tenantFixture
	keyBob, credAlice string
	logs              *RequestLogRepository
}

func newRequestLogFixture(t *testing.T) *requestLogFixture {
	t.Helper()
	f := &requestLogFixture{tenantFixture: newTenantFixture(t)}
	if err := f.db.QueryRow(`INSERT INTO gateway_api_keys (user_id, org_id, name, key_hash, key_prefix, enabled)
		VALUES ('bob', 'org_bob', 'bob-key', 'hash-bob', 'prism_sk_bob', true) RETURNING id`).Scan(&f.keyBob); err != nil {
		t.Fatal(err)
	}
	if err := f.db.QueryRow(`INSERT INTO credentials (provider_id, user_id, name, encrypted_key, key_prefix, priority, enabled, status)
		VALUES ($1, 'alice', 'ops@example.com', 'enc', 'sk-', 1, true, 'active') RETURNING id`, f.provAlice).Scan(&f.credAlice); err != nil {
		t.Fatal(err)
	}
	f.logs = NewRequestLogRepository(f.db)
	agent := "agent-1"
	for _, l := range []*models.RequestLog{
		{GatewayAPIKeyID: &f.keyAlice, ProviderID: &f.provAlice, CredentialID: &f.credAlice, Model: "gpt-4o", StatusCode: 200, LatencyMs: 100, InputTokens: 10, OutputTokens: 20, TotalTokens: 30, CostUSD: 0.5, ClientApp: "cursor"},
		{GatewayAPIKeyID: &f.keyAlice, ProviderID: &f.provAlice, Model: "gpt-4o", StatusCode: 500, LatencyMs: 300, ErrorMessage: sql.NullString{String: "upstream boom", Valid: true}, RetryCount: 1},
		{GatewayAPIKeyID: &f.keyAlice, ProviderID: &f.provAlice, Model: "claude-3", StatusCode: 200, LatencyMs: 200, InputTokens: 5, OutputTokens: 5, TotalTokens: 10, CostUSD: 0.25, AgentID: &agent, IsStream: true, TTFTMs: 40, Attempts: []byte(`[{"provider":"alice-openai"}]`)},
		{GatewayAPIKeyID: &f.keyBob, ProviderID: &f.provBob, Model: "gpt-4o", StatusCode: 200, LatencyMs: 50, TotalTokens: 1, CostUSD: 9},
	} {
		l.RequestID = uuid.NewString()
		if err := f.logs.Create(context.Background(), l); err != nil {
			t.Fatalf("create log: %v", err)
		}
	}
	return f
}

func TestRequestLogFiltersAndPaging(t *testing.T) {
	f := newRequestLogFixture(t)
	ctx := context.Background()

	cases := []struct {
		name   string
		filter LogFilter
		want   int
	}{
		{"all of alice", LogFilter{UserID: "alice"}, 3},
		{"bob", LogFilter{UserID: "bob"}, 1},
		{"by model", LogFilter{UserID: "alice", Model: "gpt-4o"}, 2},
		{"by status", LogFilter{UserID: "alice", Status: 500}, 1},
		{"by provider slug", LogFilter{UserID: "alice", Provider: "alice-openai"}, 3},
		{"other provider", LogFilter{UserID: "alice", Provider: "bob-openai"}, 0},
		{"by agent", LogFilter{UserID: "alice", AgentID: "agent-1"}, 1},
		{"search error text", LogFilter{UserID: "alice", Search: "BOOM"}, 1},
		{"search model", LogFilter{UserID: "alice", Search: "claude"}, 1},
	}
	for _, tc := range cases {
		logs, total, err := f.logs.ListWithFilter(ctx, tc.filter)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if total != tc.want || len(logs) != tc.want {
			t.Fatalf("%s: total %d, rows %d, want %d", tc.name, total, len(logs), tc.want)
		}
	}

	page, total, err := f.logs.ListWithFilter(ctx, LogFilter{UserID: "alice", Limit: 2, Offset: 2})
	if err != nil || total != 3 || len(page) != 1 {
		t.Fatalf("page 2 = %d rows of %d, %v", len(page), total, err)
	}

	logs, _, _ := f.logs.ListWithFilter(ctx, LogFilter{UserID: "alice", Model: "gpt-4o", Status: 200})
	if len(logs) != 1 || logs[0].CredentialName != "op***s@example.com" || logs[0].EstimatedCost != 0.5 {
		t.Fatalf("credential name not masked or cost not copied: %+v", logs)
	}

	byUser, err := f.logs.ListByUserID(ctx, "alice", 10, 0)
	if err != nil || len(byUser) != 3 {
		t.Fatalf("ListByUserID = %d rows, %v", len(byUser), err)
	}
}

func TestRequestLogStatsChartsAndHealth(t *testing.T) {
	f := newRequestLogFixture(t)
	ctx := context.Background()

	stats, err := f.logs.GetStats(ctx, "alice", 0, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalRequests != 3 || stats.TotalTokens != 40 || stats.InputTokens != 15 || stats.OutputTokens != 25 {
		t.Fatalf("stats = %+v", stats)
	}
	if stats.TotalEstimatedCost != 0.75 || stats.AvgLatency != 200 {
		t.Fatalf("cost/latency = %+v", stats)
	}
	if stats.ErrorRate < 33.3 || stats.ErrorRate > 33.4 || stats.FailoverRecoveryRate != 0 {
		t.Fatalf("error rate %v, failover recovery %v", stats.ErrorRate, stats.FailoverRecoveryRate)
	}
	if stats.ActiveProviders != 1 || stats.ActiveCredentials != 1 || stats.ActiveKeys != 1 {
		t.Fatalf("active counts = %+v", stats)
	}
	if byDays, err := f.logs.GetStats(ctx, "alice", 7, "", ""); err != nil || byDays.TotalRequests != 3 {
		t.Fatalf("stats by days = %+v, %v", byDays, err)
	}
	if ranged, err := f.logs.GetStats(ctx, "alice", 0, "2000-01-01", "2000-01-02"); err != nil || ranged.TotalRequests != 0 {
		t.Fatalf("stats in an empty range = %+v, %v", ranged, err)
	}

	points, err := f.logs.GetUsageChart(ctx, "alice", 30, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 2 || points[0].Model != "claude-3" || points[1].Model != "gpt-4o" || points[1].Requests != 2 || points[1].Tokens != 30 {
		t.Fatalf("usage points = %+v", points)
	}

	health, err := f.logs.GetProviderHealth(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	var mine *ProviderHealth
	for i := range health {
		if health[i].ID == f.provAlice {
			mine = &health[i]
		}
		if health[i].ID == f.provBob {
			t.Fatalf("bob's provider listed for alice: %+v", health)
		}
	}
	if mine == nil || mine.Status != "healthy" || mine.CredCount != 1 {
		t.Fatalf("alice's provider health = %+v", mine)
	}

	analytics, err := f.logs.GetLogAnalytics(ctx, "alice", 30)
	if err != nil {
		t.Fatal(err)
	}
	if analytics.TotalSpendUSD != 0.75 || analytics.AvgTTFTMs != 40 || len(analytics.Models) != 2 {
		t.Fatalf("analytics = %+v", analytics)
	}
	apps := map[string]int64{}
	for _, a := range analytics.ClientApps {
		apps[a.ClientApp] = a.Requests
	}
	if apps["cursor"] != 1 || apps["Unknown Client"] != 2 {
		t.Fatalf("client apps = %+v", analytics.ClientApps)
	}

	agentStats, err := f.logs.GetAgentStats(ctx, "alice", "agent-1", 30)
	if err != nil || agentStats.TotalRequests != 1 || agentStats.SuccessRate != 1 || agentStats.TotalTokens != 10 {
		t.Fatalf("agent stats = %+v, %v", agentStats, err)
	}
	if none, err := f.logs.GetAgentStats(ctx, "alice", "agent-none", 30); err != nil || none.TotalRequests != 0 || none.SuccessRate != 1 {
		t.Fatalf("stats for an unused agent = %+v, %v", none, err)
	}
}
