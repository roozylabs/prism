package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/models"
	"github.com/stretchr/testify/assert"
)

type mockAgentFinder struct {
	agent *models.Agent
	err   error
}

func (m *mockAgentFinder) FindForUser(ctx context.Context, userID, ref string) (*models.Agent, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.agent, nil
}

func TestAgentPolicyMiddleware_NoHeaderPasses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AgentPolicyMiddleware(&mockAgentFinder{}))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAgentPolicyMiddleware_AgentDisabledAborts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	agent := &models.Agent{
		ID:      "agent-1",
		Name:    "Disabled Agent",
		Enabled: false,
	}

	r := gin.New()
	r.Use(AgentPolicyMiddleware(&mockAgentFinder{agent: agent}))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Prism-Agent-ID", "agent-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "disabled")
}

func TestAgentPolicyMiddleware_ValidAgentSetsContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	agent := &models.Agent{
		ID:            "agent-1",
		Name:          "Gopher Agent",
		Enabled:       true,
		AllowedModels: []string{"prism-auto", "claude-sonnet-3-5"},
	}

	r := gin.New()
	r.Use(AgentPolicyMiddleware(&mockAgentFinder{agent: agent}))
	var setID, setName string
	var retrievedAgent *models.Agent
	r.GET("/test", func(c *gin.Context) {
		setID = c.GetString("agentID")
		setName = c.GetString("agentName")
		if val, ok := c.Get("agentObject"); ok {
			retrievedAgent = val.(*models.Agent)
		}
		c.String(http.StatusOK, "ok")
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Prism-Agent-ID", "agent-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "agent-1", setID)
	assert.Equal(t, "Gopher Agent", setName)
	assert.NotNil(t, retrievedAgent)
	assert.Equal(t, []string{"prism-auto", "claude-sonnet-3-5"}, retrievedAgent.AllowedModels)
}
