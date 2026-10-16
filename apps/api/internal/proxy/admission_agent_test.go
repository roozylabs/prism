package proxy

import (
	"context"
	"database/sql"
	"net/http"
	"testing"

	"github.com/roozylabs/prism/internal/models"
)

type refAgentFinder map[string]*models.Agent

func (f refAgentFinder) FindForUser(_ context.Context, _ string, ref string) (*models.Agent, error) {
	if ag, ok := f[ref]; ok {
		return ag, nil
	}
	return nil, sql.ErrNoRows
}

func TestAdmissionJudgesTheResolvedAgent(t *testing.T) {
	strict := &models.Agent{ID: "a1", Name: "strict", Enabled: true, AllowedModels: []string{"cheap-model"}}
	adm := NewAdmissionController(nil, NewAgentGovernanceEngine(refAgentFinder{"a1": strict}), nil, nil)
	ctx := context.Background()

	// The client named a second, unknown agent next to the one the gateway
	// resolved; that used to make admission look up the unknown name, find
	// nothing and allow every model.
	res, err := adm.Evaluate(ctx, AdmissionRequest{UserID: "u1", Agent: strict, AgentID: "a1", AgentName: "ghost", ModelSlug: "gpt-expensive"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Decision != AdmissionDeny || res.ErrorCode != "agent_model_denied" || res.HTTPStatus != http.StatusForbidden {
		t.Fatalf("resolved agent's allowed_models ignored: %+v", res)
	}

	if res, _ := adm.Evaluate(ctx, AdmissionRequest{UserID: "u1", Agent: strict, ModelSlug: "cheap-model"}); res.Decision != AdmissionAllow {
		t.Fatalf("allowed model denied: %+v", res)
	}
}

func TestAdmissionDeniesUnknownAgent(t *testing.T) {
	adm := NewAdmissionController(nil, NewAgentGovernanceEngine(refAgentFinder{}), nil, nil)

	// Async jobs carry only the agent id; an id that no longer resolves must
	// not run without its rules.
	res, err := adm.Evaluate(context.Background(), AdmissionRequest{UserID: "u1", AgentID: "deleted-agent", ModelSlug: "gpt-expensive"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Decision != AdmissionDeny || res.ErrorCode != "agent_not_found" {
		t.Fatalf("unknown agent admitted: %+v", res)
	}

	if res, _ := adm.Evaluate(context.Background(), AdmissionRequest{UserID: "u1", ModelSlug: "gpt-expensive"}); res.Decision != AdmissionAllow {
		t.Fatalf("request without an agent denied: %+v", res)
	}
}
