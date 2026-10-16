package proxy_test

import (
	"context"
	"testing"

	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	"github.com/stretchr/testify/assert"
)

type mockAgentFinder struct {
	agent *models.Agent
}

func (m *mockAgentFinder) FindForUser(ctx context.Context, userID, ref string) (*models.Agent, error) {
	return m.agent, nil
}

func TestAgentGovernanceEngine_DenyPrecedenceOverAllowWildcard(t *testing.T) {
	agent := &models.Agent{
		Name:          "coder_agent",
		Enabled:       true,
		AllowedModels: []string{"*", "!gpt-4o"},
		AllowedTools:  []string{"*", "deny:bash_exec"},
	}

	engine := proxy.NewAgentGovernanceEngine(&mockAgentFinder{agent: agent})

	// Model test: gpt-3.5-turbo should be allowed (matches *), but gpt-4o should be denied (matches !gpt-4o)
	res1, err := engine.ValidateAgentModelAccess(context.Background(), "u1", "coder_agent", "gpt-3.5-turbo")
	assert.NoError(t, err)
	assert.True(t, res1.ModelAllowed)

	res2, err := engine.ValidateAgentModelAccess(context.Background(), "u1", "coder_agent", "gpt-4o")
	assert.NoError(t, err)
	assert.False(t, res2.ModelAllowed)

	// Tool test: file_read allowed (*), bash_exec denied (deny:bash_exec)
	tRes1, err := engine.ValidateAgentToolAccess(context.Background(), "u1", "coder_agent", "file_read")
	assert.NoError(t, err)
	assert.True(t, tRes1.ToolAllowed)

	tRes2, err := engine.ValidateAgentToolAccess(context.Background(), "u1", "coder_agent", "bash_exec")
	assert.NoError(t, err)
	assert.False(t, tRes2.ToolAllowed)
}
