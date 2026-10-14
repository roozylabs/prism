package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/handlers"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	"github.com/roozylabs/prism/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newRoleTestRouter builds a gateway router whose governance policies deny
// every model for the given role.
func newRoleTestRouter(t *testing.T, deniedRole string, resolver handlers.MemberRoleResolver) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	orgA := "org_alpha"
	key := &models.GatewayAPIKey{ID: "key_1", UserID: "user_1", OrgID: &orgA, KeyHash: "hash_1", KeyPrefix: "gw_sk_test", Enabled: true}
	policy := models.GovernancePolicy{ID: "pol_deny_role", UserID: "user_1", Name: "Deny role", Effect: "deny", Role: deniedRole, ModelPattern: "*", Enabled: true}

	rbacEngine := proxy.NewRBACEngine(&mockPolicyFinder{policies: []models.GovernancePolicy{policy}})
	admissionCtrl := proxy.NewAdmissionController(rbacEngine, nil, nil, nil)
	orchestrator := service.NewExecutionOrchestrator(nil, admissionCtrl, nil, nil, nil, nil, nil)
	gatewayHandler := handlers.NewGatewayHandler(nil, nil, nil, nil, nil, nil, nil, rbacEngine, nil, nil, nil, orchestrator)
	if resolver != nil {
		gatewayHandler.SetMemberRoleResolver(resolver)
	}

	router := gin.New()
	rg := router.Group("/v1")
	rg.Use(middleware.GatewayAuthMiddleware(&mockKeyFinder{key: key}))
	rg.Use(middleware.TenantMiddleware())
	rg.POST("/chat/completions", gatewayHandler.ChatCompletions)
	return router
}

func postChat(router *gin.Engine, headers map[string]string) (*httptest.ResponseRecorder, map[string]interface{}) {
	body := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer gw_sk_test_key")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return w, resp
}

func errorType(t *testing.T, resp map[string]interface{}) interface{} {
	t.Helper()
	errObj, ok := resp["error"].(map[string]interface{})
	require.True(t, ok, "expected error object in response: %v", resp)
	return errObj["type"]
}

func TestGatewayRole_HeaderCannotEscapeRolePolicy(t *testing.T) {
	resolver := &mockRoleResolver{roles: map[string]string{"org_alpha/user_1": "viewer"}}
	router := newRoleTestRouter(t, "viewer", resolver)

	// A viewer claiming to be an owner must still hit the viewer deny policy.
	w, resp := postChat(router, map[string]string{"X-Prism-Role": "owner"})

	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	assert.Equal(t, "governance_policy_denied", errorType(t, resp))
}

func TestGatewayRole_HeaderCannotAdoptDeniedRole(t *testing.T) {
	resolver := &mockRoleResolver{roles: map[string]string{"org_alpha/user_1": "developer"}}
	router := newRoleTestRouter(t, "developer", resolver)

	w, resp := postChat(router, map[string]string{"X-Prism-Role": "viewer"})

	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	assert.Equal(t, "governance_policy_denied", errorType(t, resp))
}

func TestGatewayRole_NonMemberKeyOwnerForbidden(t *testing.T) {
	router := newRoleTestRouter(t, "nobody", &mockRoleResolver{roles: map[string]string{}})

	w, resp := postChat(router, map[string]string{"X-Prism-Role": "owner"})

	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	assert.Equal(t, "tenant_security_error", errorType(t, resp))
}

func TestGatewayRole_MissingResolverFailsClosed(t *testing.T) {
	router := newRoleTestRouter(t, "nobody", nil)

	w, resp := postChat(router, nil)

	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	assert.Equal(t, "tenant_security_error", errorType(t, resp))
}
