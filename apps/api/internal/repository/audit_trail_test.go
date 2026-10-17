package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/roozylabs/prism/internal/models"
)

func TestAdminAuditLogsAreOrgScopedAndFilterable(t *testing.T) {
	f := newTenantFixture(t)
	ctx := context.Background()
	r := NewAuditTrailRepository(f.db)

	rows := []*models.AuditLogItem{
		{OrganizationID: f.orgA, ActorID: "alice", ActorEmail: "alice@example.com", Action: "member.invite", Resource: "member", ResourceID: "m1", DetailsJSON: `{"role":"developer"}`, ActorIP: "203.0.113.7"},
		{OrganizationID: f.orgA, ActorID: "alice", ActorEmail: "alice@example.com", Action: "gateway_key.create", Resource: "gateway_key", ResourceID: "k1"},
		{OrganizationID: f.orgA, ActorID: "carol", ActorEmail: "carol@example.com", Action: "gateway_key.delete", Resource: "gateway_key", ResourceID: "k1", Status: "failure"},
		{OrganizationID: f.orgB, ActorID: "bob", ActorEmail: "bob@example.com", Action: "member.invite", Resource: "member", ResourceID: "m9"},
	}
	for _, item := range rows {
		if err := r.CreateAuditLog(ctx, item); err != nil {
			t.Fatalf("create %s: %v", item.Action, err)
		}
		if item.ID == "" || item.CreatedAt.IsZero() {
			t.Fatalf("defaults not filled: %+v", item)
		}
	}
	if rows[0].Status != "success" {
		t.Fatalf("status default = %q, want success", rows[0].Status)
	}

	items, total, err := r.ListAuditLogs(ctx, f.orgA, models.AuditExportRequest{}, 10, 0)
	if err != nil || total != 3 || len(items) != 3 {
		t.Fatalf("org A rows = %d of %d, %v", len(items), total, err)
	}
	var invite *models.AuditLogItem
	for i := range items {
		if items[i].OrganizationID != f.orgA {
			t.Fatalf("foreign org row listed: %+v", items[i])
		}
		if items[i].Action == "member.invite" {
			invite = &items[i]
		}
	}
	if invite == nil || invite.DetailsJSON != `{"role":"developer"}` || invite.ActorIP != "203.0.113.7" {
		t.Fatalf("details not read back: %+v", invite)
	}

	for _, tc := range []struct {
		name string
		req  models.AuditExportRequest
		want int
	}{
		{"by action substring", models.AuditExportRequest{Action: "gateway_key"}, 2},
		{"by status", models.AuditExportRequest{Status: "failure"}, 1},
		{"by actor email", models.AuditExportRequest{Search: "carol@"}, 1},
		{"by resource", models.AuditExportRequest{Search: "member"}, 1},
		{"no match", models.AuditExportRequest{Action: "nothing"}, 0},
	} {
		_, total, err := r.ListAuditLogs(ctx, f.orgA, tc.req, 10, 0)
		if err != nil || total != tc.want {
			t.Fatalf("%s: total %d (%v), want %d", tc.name, total, err, tc.want)
		}
	}

	page, total, err := r.ListAuditLogs(ctx, f.orgA, models.AuditExportRequest{}, 2, 2)
	if err != nil || total != 3 || len(page) != 1 {
		t.Fatalf("page 2 = %d rows of %d, %v", len(page), total, err)
	}
}

func TestAIAuditTrailsFilterAndScope(t *testing.T) {
	f := newTenantFixture(t)
	ctx := context.Background()
	r := NewAuditTrailRepository(f.db)

	agent := "researcher"
	trails := []*models.AIAuditTrail{
		{RequestID: "req-1", UserID: "alice", AgentName: &agent, ModelSlug: "gpt-4o", PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15, TotalCostUSD: 0.01, StatusCode: 200, SignatureHash: "sig1"},
		{RequestID: "req-2", UserID: "alice", ModelSlug: "claude-3", ComplianceStatus: "flagged", StatusCode: 403, SignatureHash: "sig2", ToolsInvoked: []string{"search"}},
		{RequestID: "req-3", UserID: "bob", ModelSlug: "gpt-4o", SignatureHash: "sig3"},
	}
	for _, tr := range trails {
		if err := r.Create(ctx, tr); err != nil {
			t.Fatalf("create %s: %v", tr.RequestID, err)
		}
		if tr.ID == "" || tr.CreatedAt.IsZero() {
			t.Fatalf("id not returned: %+v", tr)
		}
	}
	if trails[0].UserRole != "developer" || trails[0].ComplianceStatus != "compliant" || trails[0].FailoverChain == nil {
		t.Fatalf("defaults not applied: %+v", trails[0])
	}

	for _, tc := range []struct {
		name   string
		filter models.AuditTrailFilter
		want   int
	}{
		{"alice", models.AuditTrailFilter{UserID: "alice"}, 2},
		{"bob", models.AuditTrailFilter{UserID: "bob"}, 1},
		{"by agent", models.AuditTrailFilter{UserID: "alice", AgentName: "researcher"}, 1},
		{"by model", models.AuditTrailFilter{UserID: "alice", ModelSlug: "claude-3"}, 1},
		{"by compliance", models.AuditTrailFilter{UserID: "alice", ComplianceStatus: "flagged"}, 1},
		{"paged", models.AuditTrailFilter{UserID: "alice", Limit: 1, Offset: 1}, 2},
	} {
		got, total, err := r.ListWithFilter(ctx, tc.filter)
		if err != nil || total != tc.want {
			t.Fatalf("%s: total %d (%v), want %d", tc.name, total, err, tc.want)
		}
		if tc.filter.Limit > 0 && len(got) != tc.filter.Limit {
			t.Fatalf("%s: %d rows, want %d", tc.name, len(got), tc.filter.Limit)
		}
	}

	got, err := r.FindByID(ctx, trails[1].ID, "alice")
	if err != nil || got.ComplianceStatus != "flagged" || len(got.ToolsInvoked) != 1 || got.AgentName != nil {
		t.Fatalf("find = %+v, %v", got, err)
	}
	if _, err := r.FindByID(ctx, trails[1].ID, "bob"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("bob read alice's trail: %v", err)
	}
}
