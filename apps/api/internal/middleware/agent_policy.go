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

func AgentPolicyMiddleware(agents AgentFinder) gin.HandlerFunc {
	return func(c *gin.Context) {
		agentID := strings.TrimSpace(c.GetHeader("X-Prism-Agent-ID"))
		if agentID == "" {
			agentID = strings.TrimSpace(c.GetHeader("X-Agent-Name"))
		}
		if agentID == "" {
			c.Next()
			return
		}

		var ownerID string
		if key, ok := c.Get("gatewayKey"); ok {
			if gk, isKey := key.(*models.GatewayAPIKey); isKey && gk != nil {
				ownerID = gk.UserID
			}
		}
		agent, err := agents.FindForUser(c.Request.Context(), ownerID, agentID)

		if err != nil || agent == nil {
			httputil.RespondForbidden(c, "Agent identity specified in X-Prism-Agent-ID was not found", err, "AGENT_NOT_FOUND")
			return
		}

		if !agent.Enabled {
			httputil.RespondForbidden(c, "Agent specified in X-Prism-Agent-ID is disabled", nil, "AGENT_DISABLED")
			return
		}

		c.Set("agentID", agent.ID)
		c.Set("agentName", agent.Name)
		c.Set("agentObject", agent)

		c.Next()
	}
}
