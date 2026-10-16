package repository

import (
	"context"
	"errors"
	"testing"
)

func TestQuotaTargetsBelongToTheOrganization(t *testing.T) {
	db := migratedDatabase(t)
	ctx := context.Background()
	if _, err := db.Exec(`
		INSERT INTO organizations (id, name, slug) VALUES ('org1', 'o1', 'o1'), ('org2', 'o2', 'o2');
		INSERT INTO workspaces (id, org_id, name, slug) VALUES ('ws1', 'org1', 'w1', 'w1'), ('ws2', 'org2', 'w2', 'w2');
		INSERT INTO projects (id, workspace_id, name, slug) VALUES ('p1', 'ws1', 'p1', 'p1'), ('p2', 'ws2', 'p2', 'p2');`); err != nil {
		t.Fatal(err)
	}
	r := NewQuotaRepository(db)

	for _, tc := range []struct{ targetType, targetID string }{
		{"organization", "org2"},
		{"workspace", "ws2"},
		{"project", "p2"},
		{"agent", "anything"},
		{"organization", "default"},
	} {
		if _, err := r.ResolveTarget(ctx, "org1", tc.targetType, tc.targetID); !errors.Is(err, ErrQuotaTargetNotFound) {
			t.Errorf("org1 resolving %s %s: err = %v, want ErrQuotaTargetNotFound", tc.targetType, tc.targetID, err)
		}
	}

	project, err := r.ResolveTarget(ctx, "org1", "project", "p1")
	if err != nil || project.WorkspaceID == nil || *project.WorkspaceID != "ws1" {
		t.Fatalf("resolving org1's project: %+v, %v", project, err)
	}
	project.MonthlySpendLimitUSD = 5
	if err := r.UpsertQuota(ctx, project); err != nil {
		t.Fatal(err)
	}
	project.MonthlySpendLimitUSD = 7
	if err := r.UpsertQuota(ctx, project); err != nil {
		t.Fatalf("updating the same target again: %v", err)
	}
	workspace, err := r.ResolveTarget(ctx, "org1", "workspace", "ws1")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.UpsertQuota(ctx, workspace); err != nil {
		t.Fatal(err)
	}

	quotas, err := r.ListQuotas(ctx, "org1")
	if err != nil || len(quotas) != 2 {
		t.Fatalf("org1 lists %d quotas, %v; want its project and workspace", len(quotas), err)
	}
	if quotas[0].TargetType != "project" || quotas[0].TargetID != "p1" || quotas[0].MonthlySpendLimitUSD != 7 {
		t.Fatalf("project quota = %+v", quotas[0])
	}

	// A quota goes with its target.
	if _, err := db.Exec(`DELETE FROM workspaces WHERE id = 'ws1'`); err != nil {
		t.Fatal(err)
	}
	if quotas, err := r.ListQuotas(ctx, "org1"); err != nil || len(quotas) != 0 {
		t.Fatalf("after deleting the workspace org1 still has %d quotas, %v", len(quotas), err)
	}
}

func TestWorkspaceQuotaCountsWorkspaceSpend(t *testing.T) {
	db := migratedDatabase(t)
	ctx := context.Background()
	if _, err := db.Exec(`
		INSERT INTO organizations (id, name, slug) VALUES ('org1', 'o1', 'o1');
		INSERT INTO workspaces (id, org_id, name, slug) VALUES ('ws1', 'org1', 'w1', 'w1'), ('ws2', 'org1', 'w2', 'w2');
		INSERT INTO request_logs (model, status_code, latency_ms, cost_usd, org_id, workspace_id)
		VALUES ('m', 200, 1, 3, 'org1', 'ws1'), ('m', 200, 1, 4, 'org1', 'ws2');`); err != nil {
		t.Fatal(err)
	}
	r := NewQuotaRepository(db)

	// Workspace quotas used to be evaluated against the organization spend
	// query with the workspace id, which matched nothing.
	res, err := r.EvaluateQuota(ctx, "workspace", "ws1")
	if err != nil || res.MonthlySpendUSD != 3 {
		t.Fatalf("ws1 spend = %+v, %v; want 3", res, err)
	}
	res, err = r.EvaluateQuota(ctx, "organization", "org1")
	if err != nil || res.MonthlySpendUSD != 7 {
		t.Fatalf("org1 spend = %+v, %v; want 7", res, err)
	}
}
