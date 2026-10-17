package handlers

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func agentRoutes(h *AgentHandler) func(r *gin.Engine) {
	return func(r *gin.Engine) {
		r.GET("/api/agents", h.List)
		r.POST("/api/agents", h.Create)
		r.GET("/api/agents/:id", h.Get)
		r.GET("/api/agents/:id/stats", h.Stats)
		r.PUT("/api/agents/:id", h.Update)
		r.DELETE("/api/agents/:id", h.Delete)
	}
}

func TestAgentHandlerCRUDIsOwnerScoped(t *testing.T) {
	f := newOwnerFixture(t)
	h := NewAgentHandler(repository.NewAgentRepository(f.db), nil, repository.NewRequestLogRepository(f.db))
	alice := asUser("alice", nil, agentRoutes(h))
	bob := asUser("bob", nil, agentRoutes(h))

	requireStatus(t, doJSON(alice, http.MethodPost, "/api/agents", `{"displayName":"no name"}`), http.StatusBadRequest)

	// snake_case aliases and a USD budget cap are accepted; the system
	// prompt is derived when none is given.
	w := doJSON(alice, http.MethodPost, "/api/agents", `{"name":"researcher","description":"Finds papers","allowed_models":["gpt-4o"],"allowed_tools":["search"],"budget_cap_usd":1.5}`)
	requireStatus(t, w, http.StatusCreated)
	created := decodeBody(t, w)
	id, _ := created["id"].(string)
	require.NotEmpty(t, id)
	assert.Equal(t, 150.0, created["maxBudgetCents"])
	assert.Equal(t, []interface{}{"gpt-4o"}, created["allowedModels"])
	assert.Equal(t, []interface{}{"search"}, created["allowedTools"])
	assert.Equal(t, "You are researcher. Finds papers. Allowed tools: search.", created["systemPromptOverride"])
	assert.Equal(t, "active", created["status"])
	assert.Equal(t, true, created["enabled"])
	assert.Equal(t, "general", created["agentType"])

	requireStatus(t, doJSON(alice, http.MethodPost, "/api/agents", `{"name":"researcher"}`), http.StatusConflict)

	requireStatus(t, doJSON(bob, http.MethodGet, "/api/agents/"+id, ""), http.StatusNotFound)
	requireStatus(t, doJSON(bob, http.MethodGet, "/api/agents/"+id+"/stats", ""), http.StatusNotFound)
	requireStatus(t, doJSON(bob, http.MethodPut, "/api/agents/"+id, `{"name":"researcher","enabled":false}`), http.StatusNotFound)
	requireStatus(t, doJSON(bob, http.MethodDelete, "/api/agents/"+id, ""), http.StatusNotFound)
	w = doJSON(bob, http.MethodGet, "/api/agents", "")
	requireStatus(t, w, http.StatusOK)
	assert.Equal(t, "[]", w.Body.String())

	w = doJSON(alice, http.MethodGet, "/api/agents/"+id+"/stats?days=400", "")
	requireStatus(t, w, http.StatusOK)
	stats, _ := decodeBody(t, w)["data"].(map[string]interface{})
	assert.Equal(t, 0.0, stats["totalRequests"])
	assert.Equal(t, 1.0, stats["successRate"], "no requests yet means a clean success rate")

	w = doJSON(alice, http.MethodPut, "/api/agents/"+id, `{"name":"researcher","displayName":"Researcher","description":"Finds papers","systemPromptOverride":"Be brief.","maxBudgetCents":900,"enabled":false}`)
	requireStatus(t, w, http.StatusOK)
	updated := decodeBody(t, w)
	assert.Equal(t, "Researcher", updated["displayName"])
	assert.Equal(t, "Be brief.", updated["systemPromptOverride"])
	assert.Equal(t, 900.0, updated["maxBudgetCents"])
	assert.Equal(t, false, updated["enabled"])
	assert.Equal(t, []interface{}{"gpt-4o"}, updated["allowedModels"], "omitted lists are kept")

	w = doJSON(alice, http.MethodGet, "/api/agents", "")
	list := decodeList(t, w)
	var mine int
	for _, a := range list {
		if a["id"] == id {
			mine++
			assert.Equal(t, "Researcher", a["displayName"])
		}
	}
	assert.Equal(t, 1, mine, "alice's list = %v", list)

	requireStatus(t, doJSON(alice, http.MethodDelete, "/api/agents/"+id, ""), http.StatusOK)
	requireStatus(t, doJSON(alice, http.MethodGet, "/api/agents/"+id, ""), http.StatusNotFound)
}
