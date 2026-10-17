package handlers

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func policyRoutes(h *RoutingPolicyHandler) func(r *gin.Engine) {
	return func(r *gin.Engine) {
		r.GET("/api/routing-policies", h.List)
		r.POST("/api/routing-policies", h.Create)
		r.GET("/api/routing-policies/by-name/:name", h.FindByName)
		r.GET("/api/routing-policies/:id", h.Get)
		r.PUT("/api/routing-policies/:id", h.Update)
		r.DELETE("/api/routing-policies/:id", h.Delete)
		r.POST("/api/routing-policies/:id/default", h.SetDefault)
	}
}

func TestRoutingPolicyHandlerCRUDAndDefault(t *testing.T) {
	f := newOwnerFixture(t)
	h := NewRoutingPolicyHandler(repository.NewRoutingPolicyRepository(f.db))
	alice := asUser("alice", nil, policyRoutes(h))
	bob := asUser("bob", nil, policyRoutes(h))

	w := doJSON(alice, http.MethodGet, "/api/routing-policies", "")
	requireStatus(t, w, http.StatusOK)
	assert.Equal(t, "[]", w.Body.String())

	w = doJSON(alice, http.MethodPost, "/api/routing-policies", `{"name":"fast"}`)
	requireStatus(t, w, http.StatusCreated)
	fast := decodeBody(t, w)
	fastID, _ := fast["id"].(string)
	require.NotEmpty(t, fastID)
	weights, _ := fast["weights"].(map[string]interface{})
	assert.Equal(t, 0.35, weights["task_match"], "default weights applied")
	constraints, _ := fast["constraints"].(map[string]interface{})
	assert.Equal(t, 0.05, constraints["max_cost_per_request"])
	assert.Equal(t, true, fast["enabled"])
	assert.Equal(t, false, fast["isDefault"])

	// Same name again is a conflict, not a 500.
	w = doJSON(alice, http.MethodPost, "/api/routing-policies", `{"name":"fast"}`)
	requireStatus(t, w, http.StatusConflict)

	w = doJSON(alice, http.MethodPost, "/api/routing-policies", `{"name":"quality","weights":{"quality":1},"constraints":{}}`)
	requireStatus(t, w, http.StatusCreated)
	quality := decodeBody(t, w)
	qualityID, _ := quality["id"].(string)
	assert.Equal(t, map[string]interface{}{"quality": 1.0}, quality["weights"])

	// Bob neither sees nor changes alice's policies; his own "fast" is separate.
	requireStatus(t, doJSON(bob, http.MethodGet, "/api/routing-policies/"+fastID, ""), http.StatusNotFound)
	requireStatus(t, doJSON(bob, http.MethodGet, "/api/routing-policies/by-name/fast", ""), http.StatusNotFound)
	requireStatus(t, doJSON(bob, http.MethodPut, "/api/routing-policies/"+fastID, `{"enabled":false}`), http.StatusNotFound)
	requireStatus(t, doJSON(bob, http.MethodPost, "/api/routing-policies/"+fastID+"/default", ""), http.StatusNotFound)
	requireStatus(t, doJSON(bob, http.MethodDelete, "/api/routing-policies/"+fastID, ""), http.StatusNotFound)
	requireStatus(t, doJSON(bob, http.MethodPost, "/api/routing-policies", `{"name":"fast"}`), http.StatusCreated)

	w = doJSON(alice, http.MethodGet, "/api/routing-policies/by-name/fast", "")
	requireStatus(t, w, http.StatusOK)
	assert.Equal(t, fastID, decodeBody(t, w)["id"])

	// Setting a default moves the flag.
	w = doJSON(alice, http.MethodPost, "/api/routing-policies/"+fastID+"/default", "")
	requireStatus(t, w, http.StatusOK)
	assert.Equal(t, true, decodeBody(t, w)["isDefault"])
	w = doJSON(alice, http.MethodPost, "/api/routing-policies/"+qualityID+"/default", "")
	requireStatus(t, w, http.StatusOK)
	w = doJSON(alice, http.MethodGet, "/api/routing-policies", "")
	list := decodeList(t, w)
	require.Len(t, list, 2)
	assert.Equal(t, "quality", list[0]["name"], "default first")
	assert.Equal(t, true, list[0]["isDefault"])
	assert.Equal(t, false, list[1]["isDefault"])

	w = doJSON(alice, http.MethodPut, "/api/routing-policies/"+fastID, `{"name":"fastest","enabled":false,"weights":{"speed":1}}`)
	requireStatus(t, w, http.StatusOK)
	updated := decodeBody(t, w)
	assert.Equal(t, "fastest", updated["name"])
	assert.Equal(t, false, updated["enabled"])
	assert.Equal(t, map[string]interface{}{"speed": 1.0}, updated["weights"])

	requireStatus(t, doJSON(alice, http.MethodDelete, "/api/routing-policies/"+fastID, ""), http.StatusOK)
	requireStatus(t, doJSON(alice, http.MethodGet, "/api/routing-policies/"+fastID, ""), http.StatusNotFound)
	w = doJSON(alice, http.MethodGet, "/api/routing-policies", "")
	assert.Len(t, decodeList(t, w), 1)
}
