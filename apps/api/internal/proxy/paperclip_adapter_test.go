package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/stretchr/testify/assert"
)

func TestExtractPaperclipContext_HeadersPresent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req, _ := http.NewRequest("POST", "/v1/adapters/paperclip/chat/completions", nil)
	req.Header.Set("X-Paperclip-Agent-ID", "paperclip-dev-1")
	req.Header.Set("X-Paperclip-Task-ID", "task-999")
	req.Header.Set("X-Paperclip-Project-ID", "proj-42")
	req.Header.Set("X-Paperclip-Workflow-ID", "wf-100")
	c.Request = req

	agentCtx := ExtractPaperclipContext(c)
	assert.NotNil(t, agentCtx)
	assert.Equal(t, "paperclip-dev-1", agentCtx.AgentID)
	assert.Equal(t, "task-999", agentCtx.TaskID)
	assert.Equal(t, "proj-42", agentCtx.ProjectID)
	assert.Equal(t, "wf-100", agentCtx.WorkflowID)
}

func TestExtractPaperclipContext_NoHeadersNil(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req, _ := http.NewRequest("POST", "/v1/adapters/paperclip/chat/completions", nil)
	c.Request = req

	agentCtx := ExtractPaperclipContext(c)
	assert.Nil(t, agentCtx)
}

func TestPaperclipRegisterOnlyManagesItsOwnAgents(t *testing.T) {
	db := migratedTestDB(t)
	if _, err := db.Exec(`
		INSERT INTO "user" (id, name, email) VALUES ('u1', 'u1', 'u1@example.com');
		INSERT INTO agents (id, user_id, name, display_name, description, system_prompt_override, allowed_models, allowed_tools, allowed_resources)
		VALUES ('00000000-0000-0000-0000-0000000000a1', 'u1', 'support', 'support', '', '', '{gpt-cheap}', '{}', '{}');`); err != nil {
		t.Fatal(err)
	}
	adapter := NewPaperclipAdapter(repository.NewAgentRepository(db))
	ctx := context.Background()

	// A key could rewrite the allowed models of any of its owner's agents.
	_, err := adapter.RegisterPaperclipAgent(ctx, "u1", "00000000-0000-0000-0000-0000000000a1", "support", "", []string{"gpt-expensive"}, nil)
	if !errors.Is(err, ErrNotPaperclipAgent) {
		t.Fatalf("err = %v, want ErrNotPaperclipAgent", err)
	}
	var models []string
	if err := db.QueryRow(`SELECT allowed_models FROM agents WHERE id = '00000000-0000-0000-0000-0000000000a1'`).Scan(pq.Array(&models)); err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0] != "gpt-cheap" {
		t.Fatalf("regular agent's allowed models changed to %v", models)
	}

	// Nor take over a seeded system agent of the same name.
	if _, err := adapter.RegisterPaperclipAgent(ctx, "u1", "x", "research-agent", "", []string{"gpt-expensive"}, nil); !errors.Is(err, ErrNotPaperclipAgent) {
		t.Fatalf("system agent: err = %v, want ErrNotPaperclipAgent", err)
	}

	// Creating used to fail on NULL tool lists, and the agent was looked up
	// by Paperclip's own id, which never matched ours, so re-registering
	// could not update it.
	created, err := adapter.RegisterPaperclipAgent(ctx, "u1", "pc-external-1", "pc", "", []string{"m1"}, nil)
	if err != nil {
		t.Fatalf("creating a Paperclip agent: %v", err)
	}
	updated, err := adapter.RegisterPaperclipAgent(ctx, "u1", "pc-external-1", "pc", "", []string{"m2"}, nil)
	if err != nil || updated.ID != created.ID || updated.AllowedModels[0] != "m2" {
		t.Fatalf("re-registering updates the same agent: %+v, %v", updated, err)
	}
}
