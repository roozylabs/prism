package security_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/handlers"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestTenantContext_StringKeyAliases(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	orgID := "org_company_a"
	wsID := "ws_eng"
	projID := "proj_llm"

	r.Use(func(c *gin.Context) {
		c.Set("gatewayKey", &models.GatewayAPIKey{
			ID:          "key_123",
			OrgID:       &orgID,
			WorkspaceID: &wsID,
			ProjectID:   &projID,
		})
		c.Next()
	})
	r.Use(middleware.TenantMiddleware())

	r.GET("/test/context", func(c *gin.Context) {
		tc := middleware.GetTenantContext(c)
		c.JSON(http.StatusOK, gin.H{
			"orgFromStruct":      tc.OrgID,
			"orgFromSnake":       c.GetString("organization_id"),
			"orgFromCamel":       c.GetString("organizationId"),
			"wsFromSnake":        c.GetString("workspace_id"),
			"projFromSnake":      c.GetString("project_id"),
		})
	})

	req, _ := http.NewRequest("GET", "/test/context", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), `"orgFromStruct":"org_company_a"`)
	assert.Contains(t, resp.Body.String(), `"orgFromSnake":"org_company_a"`)
	assert.Contains(t, resp.Body.String(), `"orgFromCamel":"org_company_a"`)
	assert.Contains(t, resp.Body.String(), `"wsFromSnake":"ws_eng"`)
	assert.Contains(t, resp.Body.String(), `"projFromSnake":"proj_llm"`)
}

func TestQuotaHandler_CrossOrgUpdateForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := handlers.NewQuotaHandler(nil)

	r.Use(func(c *gin.Context) {
		c.Set("organizationId", "org_tenant_A")
		c.Next()
	})

	r.PUT("/api/quotas/:target_type/:target_id", handler.Update)
	r.GET("/api/quotas/:target_type/:target_id", handler.Get)

	// Tenant A tries to update Quota for Tenant B
	payload := `{"monthlySpendLimitUsd": 500.0}`
	req, _ := http.NewRequest("PUT", "/api/quotas/organization/org_tenant_B", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusForbidden, resp.Code)
	assert.Contains(t, resp.Body.String(), "cannot access the quota of a different organization")

	// Reading it, which reports Tenant B's spend, is refused too.
	req, _ = http.NewRequest("GET", "/api/quotas/organization/org_tenant_B", nil)
	resp = httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusForbidden, resp.Code)
}
