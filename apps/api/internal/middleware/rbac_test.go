package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/authz"
	"github.com/roozylabs/prism/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockPermFinder struct {
	perms map[string][]string // userID -> permissions in org_1
	err   error
}

func (m *mockPermFinder) GetUserPermissions(ctx context.Context, userID, orgID string) ([]string, string, error) {
	if m.err != nil {
		return nil, "", m.err
	}
	perms, ok := m.perms[userID]
	if !ok || orgID != "org_1" {
		return nil, "", errors.New("not a member")
	}
	return perms, "custom", nil
}

type mockWsFinder struct{}

func (mockWsFinder) GetWorkspaceMemberRole(ctx context.Context, wsID, userID string) (string, error) {
	return "", errors.New("not a member")
}

func newPermRouter(engine *authz.AuthorizationEngine, code string, setup func(c *gin.Context)) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/target", func(c *gin.Context) {
		setup(c)
		c.Next()
	}, RequirePermission(engine, code), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func asUser(userID, orgID string) func(c *gin.Context) {
	return func(c *gin.Context) {
		c.Set("userId", userID)
		c.Set("organization_id", orgID)
	}
}

func serve(r *gin.Engine) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/target", nil))
	return w
}

func errorCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var resp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp), w.Body.String())
	return resp.Error.Code
}

func TestRequirePermission_NilEngineFailsClosed(t *testing.T) {
	r := newPermRouter(nil, "credential:read", asUser("user_1", "org_1"))

	w := serve(r)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, "AUTHZ_UNAVAILABLE", errorCode(t, w))
}

func TestRequirePermission_Unauthenticated(t *testing.T) {
	engine := authz.NewAuthorizationEngine(&mockPermFinder{}, mockWsFinder{})
	r := newPermRouter(engine, "credential:read", func(c *gin.Context) {})

	w := serve(r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequirePermission_ForbiddenCases(t *testing.T) {
	finder := &mockPermFinder{perms: map[string][]string{
		"viewer":    {"credential:read", "mcp:read", "audit:read"},
		"developer": {"credential:read", "credential:create", "credential:update", "mcp:create", "api_key:create"},
	}}
	engine := authz.NewAuthorizationEngine(finder, mockWsFinder{})

	cases := []struct {
		name  string
		code  string
		setup func(c *gin.Context)
	}{
		{"viewer cannot reveal credentials", "credential:update", asUser("viewer", "org_1")},
		{"viewer cannot create MCP servers", "mcp:create", asUser("viewer", "org_1")},
		{"viewer cannot create gateway keys", "api_key:create", asUser("viewer", "org_1")},
		{"developer cannot delete credentials", "credential:delete", asUser("developer", "org_1")},
		{"developer cannot update quotas", "quota:update", asUser("developer", "org_1")},
		{"developer cannot manage billing", "billing:manage", asUser("developer", "org_1")},
		{"developer cannot update settings", "organization:update", asUser("developer", "org_1")},
		{"developer cannot change governance", "governance:create", asUser("developer", "org_1")},
		{"developer cannot export audit", "audit:export", asUser("developer", "org_1")},
		{"non-member denied", "credential:read", asUser("stranger", "org_1")},
		{"member of another org denied", "credential:read", asUser("developer", "org_2")},
		{"workspace non-member denied", "credential:read", func(c *gin.Context) {
			asUser("developer", "org_1")(c)
			c.Set("workspace_id", "ws_other")
		}},
		{"gateway key has no admin permissions", "credential:read", func(c *gin.Context) {
			org := "org_1"
			asUser("developer", "org_1")(c)
			c.Set("gatewayKey", &models.GatewayAPIKey{ID: "key_1", UserID: "developer", OrgID: &org})
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := serve(newPermRouter(engine, tc.code, tc.setup))
			assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
			assert.Equal(t, "PERMISSION_DENIED", errorCode(t, w))
		})
	}
}

func TestRequirePermission_PermissionLookupErrorFailsClosed(t *testing.T) {
	engine := authz.NewAuthorizationEngine(&mockPermFinder{err: errors.New("db down")}, mockWsFinder{})
	w := serve(newPermRouter(engine, "credential:read", asUser("developer", "org_1")))

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequirePermission_Allowed(t *testing.T) {
	finder := &mockPermFinder{perms: map[string][]string{"developer": {"credential:update"}}}
	engine := authz.NewAuthorizationEngine(finder, mockWsFinder{})

	w := serve(newPermRouter(engine, "credential:update", asUser("developer", "org_1")))

	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}
