package proxy

import (
	"context"
	"errors"
	"testing"

	"github.com/roozylabs/prism/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type agentFinderMock struct {
	agent *models.Agent
	err   error
}

func (m *agentFinderMock) FindForUser(ctx context.Context, userID, ref string) (*models.Agent, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.agent == nil {
		return nil, errors.New("agent not found")
	}
	return m.agent, nil
}

func TestAgentGovernanceValidateModelAccess(t *testing.T) {
	ag := &models.Agent{
		Name:          "dev-agent",
		AllowedModels: []string{"gpt-4o", "claude-sonnet"},
		AllowedTools:  []string{"search_web"},
		Enabled:       true,
	}

	engine := NewAgentGovernanceEngine(&agentFinderMock{agent: ag})

	// Test allowed model
	res, err := engine.ValidateAgentModelAccess(context.Background(), "u1", "dev-agent", "gpt-4o")
	require.NoError(t, err)
	assert.True(t, res.ModelAllowed)

	// Test denied model
	res, err = engine.ValidateAgentModelAccess(context.Background(), "u1", "dev-agent", "deepseek-coder")
	require.NoError(t, err)
	assert.False(t, res.ModelAllowed)
	assert.Contains(t, res.Reason, "not permitted")
}

func TestAgentGovernanceValidateToolAccess(t *testing.T) {
	ag := &models.Agent{
		Name:         "dev-agent",
		AllowedTools: []string{"search_web"},
		Enabled:      true,
	}

	engine := NewAgentGovernanceEngine(&agentFinderMock{agent: ag})

	// Test allowed tool
	res, err := engine.ValidateAgentToolAccess(context.Background(), "u1", "dev-agent", "search_web")
	require.NoError(t, err)
	assert.True(t, res.ToolAllowed)

	// Test denied tool
	res, err = engine.ValidateAgentToolAccess(context.Background(), "u1", "dev-agent", "execute_sql")
	require.NoError(t, err)
	assert.False(t, res.ToolAllowed)
	assert.Contains(t, res.Reason, "not permitted")
}
