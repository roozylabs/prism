package handlers

import (
	"database/sql"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedRequestLogs writes three requests for alice (one failed, one by an
// agent) and one for bob, each through their own gateway key.
func seedRequestLogs(t *testing.T, f *ownerFixture) {
	t.Helper()
	var keyAlice, keyBob string
	require.NoError(t, f.db.QueryRow(`INSERT INTO gateway_api_keys (user_id, org_id, name, key_hash, key_prefix, enabled)
		VALUES ('alice', 'org_alice', 'alice-key', 'hash-alice', 'prism_sk_alice', true) RETURNING id`).Scan(&keyAlice))
	require.NoError(t, f.db.QueryRow(`INSERT INTO gateway_api_keys (user_id, name, key_hash, key_prefix, enabled)
		VALUES ('bob', 'bob-key', 'hash-bob', 'prism_sk_bob', true) RETURNING id`).Scan(&keyBob))
	logs := repository.NewRequestLogRepository(f.db)
	agent := "agent-1"
	for _, l := range []*models.RequestLog{
		{GatewayAPIKeyID: &keyAlice, ProviderID: &f.provAlice, Model: "gpt-4o", StatusCode: 200, LatencyMs: 100, InputTokens: 10, OutputTokens: 20, TotalTokens: 30, CostUSD: 0.5, ClientApp: "cursor"},
		{GatewayAPIKeyID: &keyAlice, ProviderID: &f.provAlice, Model: "gpt-4o", StatusCode: 500, LatencyMs: 300, ErrorMessage: sql.NullString{String: "upstream boom", Valid: true}},
		{GatewayAPIKeyID: &keyAlice, ProviderID: &f.provAlice, Model: "claude-3", StatusCode: 200, LatencyMs: 200, TotalTokens: 10, CostUSD: 0.25, AgentID: &agent, TTFTMs: 40},
		{GatewayAPIKeyID: &keyBob, ProviderID: &f.provBob, Model: "gpt-4o", StatusCode: 200, LatencyMs: 50, CostUSD: 9},
	} {
		l.RequestID = uuid.NewString()
		require.NoError(t, logs.Create(t.Context(), l))
	}
}

func TestLogsHandlerListsOwnRequestsWithFilters(t *testing.T) {
	f := newOwnerFixture(t)
	seedRequestLogs(t, f)
	h := NewLogsHandler(repository.NewRequestLogRepository(f.db))
	routes := func(r *gin.Engine) {
		r.GET("/api/logs", h.List)
		r.GET("/api/analytics/logs", h.GetAnalytics)
	}
	alice := asUser("alice", nil, routes)
	bob := asUser("bob", nil, routes)

	for query, want := range map[string]float64{
		"":                       3,
		"?model=gpt-4o":          2,
		"?status=500":            1,
		"?search=boom":           1,
		"?agentId=agent-1":       1,
		"?agent_id=agent-1":      1,
		"?provider=alice-openai": 3,
		"?provider=bob-openai":   0,
	} {
		w := doJSON(alice, http.MethodGet, "/api/logs"+query, "")
		requireStatus(t, w, http.StatusOK)
		assert.Equal(t, want, decodeBody(t, w)["total"], query)
	}

	w := doJSON(alice, http.MethodGet, "/api/logs?limit=2&page=2", "")
	page := decodeBody(t, w)
	assert.Equal(t, 3.0, page["total"])
	assert.Equal(t, 2.0, page["totalPages"])
	assert.Len(t, page["data"], 1)

	w = doJSON(alice, http.MethodGet, "/api/logs?status=500", "")
	rows, _ := decodeBody(t, w)["data"].([]interface{})
	require.Len(t, rows, 1)
	assert.Equal(t, "upstream boom", rows[0].(map[string]interface{})["errorMessage"])

	w = doJSON(bob, http.MethodGet, "/api/logs", "")
	assert.Equal(t, 1.0, decodeBody(t, w)["total"])

	w = doJSON(alice, http.MethodGet, "/api/analytics/logs?days=7", "")
	requireStatus(t, w, http.StatusOK)
	data, _ := decodeBody(t, w)["data"].(map[string]interface{})
	assert.Equal(t, 0.75, data["totalSpendUsd"])
	assert.Len(t, data["models"], 2)
	assert.Len(t, data["clientApps"], 2)
}

func TestDashboardHandlerStatsUsageAndHealth(t *testing.T) {
	f := newOwnerFixture(t)
	seedRequestLogs(t, f)
	h := NewDashboardHandler(repository.NewRequestLogRepository(f.db), nil)
	routes := func(r *gin.Engine) {
		r.GET("/dashboard/stats", h.GetStats)
		r.GET("/dashboard/usage", h.GetUsageChart)
		r.GET("/dashboard/health", h.GetProviderHealth)
	}
	alice := asUser("alice", &f.tenant, routes)
	bob := asUser("bob", nil, routes)

	w := doJSON(alice, http.MethodGet, "/dashboard/stats", "")
	requireStatus(t, w, http.StatusOK)
	stats := decodeBody(t, w)
	assert.Equal(t, 3.0, stats["totalRequests"])
	assert.Equal(t, 40.0, stats["totalTokens"])
	assert.Equal(t, 0.75, stats["totalEstimatedCost"])
	assert.Equal(t, 1.0, stats["activeKeys"])
	assert.Equal(t, 1.0, stats["activeProviders"])
	assert.InDelta(t, 33.33, stats["errorRate"], 0.01)

	w = doJSON(alice, http.MethodGet, "/dashboard/stats?days=7", "")
	assert.Equal(t, 3.0, decodeBody(t, w)["totalRequests"])
	w = doJSON(alice, http.MethodGet, "/dashboard/stats?startDate=2000-01-01&endDate=2000-01-31", "")
	assert.Equal(t, 0.0, decodeBody(t, w)["totalRequests"])
	requireStatus(t, doJSON(alice, http.MethodGet, "/dashboard/stats?startDate=soon", ""), http.StatusBadRequest)
	requireStatus(t, doJSON(alice, http.MethodGet, "/dashboard/usage?startDate=2026-02-01&endDate=2026-01-01", ""), http.StatusBadRequest)

	w = doJSON(bob, http.MethodGet, "/dashboard/stats", "")
	assert.Equal(t, 1.0, decodeBody(t, w)["totalRequests"])

	w = doJSON(alice, http.MethodGet, "/dashboard/usage", "")
	requireStatus(t, w, http.StatusOK)
	points := decodeList(t, w)
	require.Len(t, points, 2)
	assert.Equal(t, "claude-3", points[0]["model"])
	assert.Equal(t, 2.0, points[1]["requests"])
	w = doJSON(alice, http.MethodGet, "/dashboard/usage?startDate=2000-01-01&endDate=2000-01-02", "")
	assert.Equal(t, "[]", w.Body.String(), "no points is an empty list")

	w = doJSON(alice, http.MethodGet, "/dashboard/health", "")
	requireStatus(t, w, http.StatusOK)
	health := decodeList(t, w)
	var mine map[string]interface{}
	for _, p := range health {
		if p["name"] == "alice-openai" {
			mine = p
		}
		assert.NotEqual(t, "bob-openai", p["name"], "bob's provider must not be listed")
	}
	require.NotNil(t, mine, "alice's provider missing from %v", health)
	assert.Equal(t, "healthy", mine["status"])
	assert.Equal(t, float64(proxy.DefaultProviderHealthScore), mine["health_score"], "no health store means the default score")
}
