package middleware

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/models"
)

// AgentFinder resolves an agent id or name to one the caller may use: their
// own, or a system agent.
type AgentFinder interface {
	FindForUser(ctx context.Context, userID, ref string) (*models.Agent, error)
}

// agentHeaders are the request headers that name an agent, in order of
// precedence.
var agentHeaders = []string{"X-Prism-Agent-ID", "X-Prism-Agent-Name", "X-Agent-Name"}

func AgentPolicyMiddleware(agents AgentFinder) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !ResolveAgent(c, agents) {
			return
		}
		c.Next()
	}
}

// ResolveAgent looks up the agent named by the request headers for the
// gateway key's owner and stores it in the context. A named agent that does
// not exist or is disabled ends the request with 403 and returns false.
func ResolveAgent(c *gin.Context, agents AgentFinder) bool {
	var ref string
	for _, h := range agentHeaders {
		if ref = strings.TrimSpace(c.GetHeader(h)); ref != "" {
			break
		}
	}
	if ref == "" {
		return true
	}

	var ownerID string
	if key, ok := c.Get("gatewayKey"); ok {
		if gk, isKey := key.(*models.GatewayAPIKey); isKey && gk != nil {
			ownerID = gk.UserID
		}
	}
	if agents == nil {
		httputil.RespondForbidden(c, "Agent identity specified in X-Prism-Agent-ID was not found", nil, "AGENT_NOT_FOUND")
		return false
	}
	agent, err := agents.FindForUser(c.Request.Context(), ownerID, ref)
	if err != nil || agent == nil {
		httputil.RespondForbidden(c, "Agent identity specified in X-Prism-Agent-ID was not found", err, "AGENT_NOT_FOUND")
		return false
	}

	if !agent.Enabled {
		httputil.RespondForbidden(c, "Agent specified in X-Prism-Agent-ID is disabled", nil, "AGENT_DISABLED")
		return false
	}

	c.Set("agentID", agent.ID)
	c.Set("agentName", agent.Name)
	c.Set("agentObject", agent)
	return true
}

// AgentFromContext returns the agent ResolveAgent stored, or nil when the
// request named none.
func AgentFromContext(c *gin.Context) *models.Agent {
	if v, ok := c.Get("agentObject"); ok {
		if ag, isAgent := v.(*models.Agent); isAgent {
			return ag
		}
	}
	return nil
}
