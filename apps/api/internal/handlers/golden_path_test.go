package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/handlers"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/roozylabs/prism/internal/service"
)

type mockKeyFinder struct {
	key *models.GatewayAPIKey
}

func (m *mockKeyFinder) FindByKeyHash(ctx context.Context, keyHash string) (*models.GatewayAPIKey, error) {
	return m.key, nil
}

type mockRoleResolver struct {
	roles map[string]string // orgID/userID -> role slug
}

func (m *mockRoleResolver) GetOrganizationMemberRole(ctx context.Context, orgID, userID string) (string, error) {
	if role, ok := m.roles[orgID+"/"+userID]; ok {
		return role, nil
	}
	return "", repository.ErrNotMember
}

type mockPolicyFinder struct {
	policies []models.GovernancePolicy
}

func (m *mockPolicyFinder) ListByUserID(ctx context.Context, userID string) ([]models.GovernancePolicy, error) {
	return m.policies, nil
}

func TestGoldenPathTenantSecurity_CrossOrgHeaderRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)

	orgA := "org_alpha"
	key := &models.GatewayAPIKey{
		ID:        "key_1",
		UserID:    "user_1",
		OrgID:     &orgA,
		KeyHash:   "hash_1",
		KeyPrefix: "gw_sk_test",
		Enabled:   true,
	}

	keyFinder := &mockKeyFinder{key: key}
	rbacEngine := proxy.NewRBACEngine(&mockPolicyFinder{})
	admissionCtrl := proxy.NewAdmissionController(rbacEngine, nil, nil, nil)
	orchestrator := service.NewExecutionOrchestrator(nil, admissionCtrl, nil, nil, nil, nil, nil)
	gatewayHandler := handlers.NewGatewayHandler(nil, nil, nil, nil, nil, nil, nil, rbacEngine, nil, nil, nil, orchestrator)
	gatewayHandler.SetMemberRoleResolver(&mockRoleResolver{roles: map[string]string{"org_alpha/user_1": "developer"}})

	router := gin.New()
	rg := router.Group("/v1")
	rg.Use(middleware.GatewayAuthMiddleware(keyFinder))
	rg.Use(middleware.TenantMiddleware())
	rg.POST("/chat/completions", gatewayHandler.ChatCompletions)

	// Attempt spoofing X-Prism-Org-ID: org_victim when key belongs to org_alpha
	reqBody := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}`)
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBuffer(reqBody))
	httpReq.Header.Set("Authorization", "Bearer gw_sk_test_key")
	httpReq.Header.Set("X-Prism-Org-ID", "org_victim")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httpReq)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected HTTP 403 Forbidden on tenant spoofing, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON error: %v", err)
	}

	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error object in response")
	}

	if errObj["type"] != "tenant_security_error" {
		t.Fatalf("expected error type 'tenant_security_error', got %v", errObj["type"])
	}
}

func TestGoldenPathDeniedByRBAC_ZeroProviderCalls(t *testing.T) {
	gin.SetMode(gin.TestMode)

	orgA := "org_alpha"
	key := &models.GatewayAPIKey{
		ID:        "key_1",
		UserID:    "user_1",
		OrgID:     &orgA,
		KeyHash:   "hash_1",
		KeyPrefix: "gw_sk_test",
		Enabled:   true,
	}

	policy := models.GovernancePolicy{
		ID:           "pol_deny_gpt4",
		UserID:       "user_1",
		Name:         "Deny GPT-4",
		Effect:       "deny",
		ModelPattern: "gpt-4*",
		Enabled:      true,
		CreatedAt:    time.Now(),
	}

	keyFinder := &mockKeyFinder{key: key}
	policyFinder := &mockPolicyFinder{policies: []models.GovernancePolicy{policy}}
	rbacEngine := proxy.NewRBACEngine(policyFinder)
	admissionCtrl := proxy.NewAdmissionController(rbacEngine, nil, nil, nil)
	orchestrator := service.NewExecutionOrchestrator(nil, admissionCtrl, nil, nil, nil, nil, nil)
	gatewayHandler := handlers.NewGatewayHandler(nil, nil, nil, nil, nil, nil, nil, rbacEngine, nil, nil, nil, orchestrator)
	gatewayHandler.SetMemberRoleResolver(&mockRoleResolver{roles: map[string]string{"org_alpha/user_1": "developer"}})

	router := gin.New()
	rg := router.Group("/v1")
	rg.Use(middleware.GatewayAuthMiddleware(keyFinder))
	rg.Use(middleware.TenantMiddleware())
	rg.POST("/chat/completions", gatewayHandler.ChatCompletions)

	reqBody := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"Secrets"}]}`)
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBuffer(reqBody))
	httpReq.Header.Set("Authorization", "Bearer gw_sk_test_key")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httpReq)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected HTTP 403 Forbidden when RBAC denies request, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	errObj := resp["error"].(map[string]interface{})
	if errObj["type"] != "governance_policy_denied" {
		t.Fatalf("expected error type 'governance_policy_denied', got %v", errObj["type"])
	}
}

func TestSDKAndCLIGovernanceConsistency(t *testing.T) {
	gin.SetMode(gin.TestMode)

	orgA := "org_alpha"
	key := &models.GatewayAPIKey{
		ID:        "key_1",
		UserID:    "user_1",
		OrgID:     &orgA,
		KeyHash:   "hash_1",
		KeyPrefix: "gw_sk_test",
		Enabled:   true,
	}

	policy := models.GovernancePolicy{
		ID:           "pol_deny_all",
		UserID:       "user_1",
		Name:         "Deny All",
		Effect:       "deny",
		ModelPattern: "*",
		Enabled:      true,
	}

	keyFinder := &mockKeyFinder{key: key}
	policyFinder := &mockPolicyFinder{policies: []models.GovernancePolicy{policy}}
	rbacEngine := proxy.NewRBACEngine(policyFinder)
	admissionCtrl := proxy.NewAdmissionController(rbacEngine, nil, nil, nil)
	orchestrator := service.NewExecutionOrchestrator(nil, admissionCtrl, nil, nil, nil, nil, nil)
	gatewayHandler := handlers.NewGatewayHandler(nil, nil, nil, nil, nil, nil, nil, rbacEngine, nil, nil, nil, orchestrator)
	gatewayHandler.SetMemberRoleResolver(&mockRoleResolver{roles: map[string]string{"org_alpha/user_1": "developer"}})

	router := gin.New()
	rg := router.Group("/v1")
	rg.Use(middleware.GatewayAuthMiddleware(keyFinder))
	rg.Use(middleware.TenantMiddleware())
	rg.POST("/chat/completions", gatewayHandler.ChatCompletions)

	userAgents := []string{"Prism-Node-SDK/2.2.0", "OpenCode/0.1.0", "Python-requests/2.28"}

	for _, ua := range userAgents {
		reqBody := []byte(`{"model":"claude-3-5-sonnet","messages":[{"role":"user","content":"Test"}]}`)
		httpReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBuffer(reqBody))
		httpReq.Header.Set("Authorization", "Bearer gw_sk_test_key")
		httpReq.Header.Set("User-Agent", ua)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, httpReq)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected HTTP 403 Forbidden for User-Agent %s, got %d", ua, w.Code)
		}
	}
}
