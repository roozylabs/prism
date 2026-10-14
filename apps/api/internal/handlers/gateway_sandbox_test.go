package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/roozylabs/prism/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sandboxKeysMock struct {
	byPrefix map[string]*models.GatewayAPIKey
}

func (m *sandboxKeysMock) FindByKeyPrefix(ctx context.Context, keyPrefix string) (*models.GatewayAPIKey, error) {
	if k, ok := m.byPrefix[keyPrefix]; ok {
		return k, nil
	}
	return nil, repository.ErrNotMember
}

func (m *sandboxKeysMock) ListByUserID(ctx context.Context, userID string) ([]models.GatewayAPIKey, error) {
	var keys []models.GatewayAPIKey
	for _, k := range m.byPrefix {
		if k.UserID == userID {
			keys = append(keys, *k)
		}
	}
	return keys, nil
}

type sandboxRolesMock struct{}

func (sandboxRolesMock) GetOrganizationMemberRole(ctx context.Context, orgID, userID string) (string, error) {
	return "developer", nil
}

type sandboxPoliciesMock struct{}

func (sandboxPoliciesMock) ListByUserID(ctx context.Context, userID string) ([]models.GovernancePolicy, error) {
	// Deny everything so a request that reaches admission stops there without
	// touching a provider; the 403 proves which key's owner was evaluated.
	return []models.GovernancePolicy{{ID: "pol_deny_" + userID, UserID: userID, Name: "Deny all", Effect: "deny", ModelPattern: "*", Enabled: true}}, nil
}

func newSandboxRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	orgVictim, orgAttacker := "org_victim", "org_attacker"
	keys := &sandboxKeysMock{byPrefix: map[string]*models.GatewayAPIKey{
		"gw_sk_victim":   {ID: "key_victim", UserID: "user_victim", OrgID: &orgVictim, KeyPrefix: "gw_sk_victim", Enabled: true},
		"gw_sk_attacker": {ID: "key_attacker", UserID: "user_attacker", OrgID: &orgAttacker, KeyPrefix: "gw_sk_attacker", Enabled: true},
	}}

	rbacEngine := proxy.NewRBACEngine(sandboxPoliciesMock{})
	orchestrator := service.NewExecutionOrchestrator(nil, proxy.NewAdmissionController(rbacEngine, nil, nil, nil), nil, nil, nil, nil, nil)
	h := NewGatewayHandler(nil, nil, nil, nil, nil, nil, nil, rbacEngine, nil, nil, nil, orchestrator)
	h.sandboxKeys = keys
	h.SetMemberRoleResolver(sandboxRolesMock{})

	r := gin.New()
	asAttacker := func(c *gin.Context) {
		c.Set("userId", "user_attacker")
		c.Next()
	}
	r.POST("/sandbox/chat/completions", asAttacker, h.SandboxChatCompletions)
	r.POST("/sandbox/chat/completions/async", asAttacker, h.AsyncSandboxChatCompletions)
	r.POST("/anonymous/sandbox/chat/completions", h.SandboxChatCompletions)
	return r
}

func postSandbox(r *gin.Engine, path, prefix string) *httptest.ResponseRecorder {
	body := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}`)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBuffer(body))
	if prefix != "" {
		req.Header.Set("X-Sandbox-Key-Prefix", prefix)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestSandbox_OtherUsersKeyPrefixRejected(t *testing.T) {
	r := newSandboxRouter(t)

	for _, path := range []string{"/sandbox/chat/completions", "/sandbox/chat/completions/async"} {
		w := postSandbox(r, path, "gw_sk_victim")
		assert.Equal(t, http.StatusUnauthorized, w.Code, "%s: %s", path, w.Body.String())
		assert.Contains(t, w.Body.String(), "Invalid API key prefix", path)
	}
}

func TestSandbox_ForeignPrefixIndistinguishableFromUnknown(t *testing.T) {
	r := newSandboxRouter(t)

	foreign := postSandbox(r, "/sandbox/chat/completions", "gw_sk_victim")
	unknown := postSandbox(r, "/sandbox/chat/completions", "gw_sk_does_not_exist")

	assert.Equal(t, unknown.Code, foreign.Code)
	assert.JSONEq(t, unknown.Body.String(), foreign.Body.String())
}

func TestSandbox_OwnKeyPrefixUsed(t *testing.T) {
	r := newSandboxRouter(t)

	w := postSandbox(r, "/sandbox/chat/completions", "gw_sk_attacker")

	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	var resp map[string]map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "governance_policy_denied", resp["error"]["type"])
	assert.Contains(t, w.Body.String(), "pol_deny_user_attacker")
}

func TestSandbox_DefaultsToCallersOwnKey(t *testing.T) {
	r := newSandboxRouter(t)

	w := postSandbox(r, "/sandbox/chat/completions", "")

	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "pol_deny_user_attacker")
}

func TestSandbox_NoSessionUserRejected(t *testing.T) {
	r := newSandboxRouter(t)

	w := postSandbox(r, "/anonymous/sandbox/chat/completions", "gw_sk_victim")

	assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
}
