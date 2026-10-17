package handlers

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/roozylabs/prism/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOnboardingCreatesOrgWorkspaceMembershipAndKey(t *testing.T) {
	f := newOwnerFixture(t)
	keys := repository.NewGatewayKeyRepository(f.db)
	h := NewOnboardingHandler(f.db, repository.NewUserRepository(f.db), keys)
	routes := func(r *gin.Engine) { r.POST("/api/onboarding", h.Complete) }
	anon := asUser("", nil, routes)
	bob := asUser("bob", nil, routes)

	requireStatus(t, doJSON(anon, http.MethodPost, "/api/onboarding", `{"organizationName":"Acme","workspaceName":"Main","gatewayKeyName":"first"}`), http.StatusUnauthorized)
	for _, body := range []string{
		`{"workspaceName":"Main","gatewayKeyName":"first"}`,
		`{"organizationName":"  ","workspaceName":"Main","gatewayKeyName":"first"}`,
		`{"organizationName":"Acme","gatewayKeyName":"first"}`,
		`{"organizationName":"Acme","workspaceName":"Main"}`,
		`not json`,
	} {
		requireStatus(t, doJSON(bob, http.MethodPost, "/api/onboarding", body), http.StatusBadRequest)
	}

	w := doJSON(bob, http.MethodPost, "/api/onboarding", `{"organizationName":"Acme Labs","workspaceName":"Main Space","gatewayKeyName":"first","primaryRole":"developer"}`)
	requireStatus(t, w, http.StatusOK)
	resp := decodeBody(t, w)
	orgID, _ := resp["organizationId"].(string)
	wsID, _ := resp["workspaceId"].(string)
	rawKey, _ := resp["apiKey"].(string)
	assert.True(t, strings.HasPrefix(orgID, "org_"), orgID)
	assert.True(t, strings.HasPrefix(wsID, "ws_"), wsID)
	assert.True(t, strings.HasPrefix(rawKey, utils.GatewayKeyPrefix), rawKey)
	assert.Equal(t, "Main Space", resp["workspaceName"])
	assert.Equal(t, "developer", resp["primaryRole"])

	var orgSlug, wsSlug string
	require.NoError(t, f.db.QueryRow(`SELECT slug FROM organizations WHERE id = $1`, orgID).Scan(&orgSlug))
	require.NoError(t, f.db.QueryRow(`SELECT slug FROM workspaces WHERE id = $1 AND org_id = $2`, wsID, orgID).Scan(&wsSlug))
	assert.True(t, strings.HasPrefix(orgSlug, "acme-labs-"), orgSlug)
	assert.True(t, strings.HasPrefix(wsSlug, "main-space-"), wsSlug)

	// The creator owns the organization whatever their job title says.
	var roleSlug string
	require.NoError(t, f.db.QueryRow(`SELECT r.slug FROM organization_members m JOIN roles r ON r.id = m.role_id WHERE m.org_id = $1 AND m.user_id = 'bob'`, orgID).Scan(&roleSlug))
	assert.Equal(t, "owner", roleSlug)
	var wsRole string
	require.NoError(t, f.db.QueryRow(`SELECT role FROM workspace_members WHERE workspace_id = $1 AND user_id = 'bob'`, wsID).Scan(&wsRole))
	assert.Equal(t, "admin", wsRole)

	var onboarded bool
	var userOrg, primaryRole string
	require.NoError(t, f.db.QueryRow(`SELECT is_onboarded, org_id, primary_role FROM "user" WHERE id = 'bob'`).Scan(&onboarded, &userOrg, &primaryRole))
	assert.True(t, onboarded)
	assert.Equal(t, orgID, userOrg)
	assert.Equal(t, "developer", primaryRole)

	key, err := keys.FindByKeyHash(t.Context(), utils.HashSHA256(rawKey))
	require.NoError(t, err)
	assert.Equal(t, "first", key.Name)
	assert.Equal(t, "bob", key.UserID)
	require.NotNil(t, key.OrgID)
	assert.Equal(t, orgID, *key.OrgID)
	assert.Equal(t, 600, key.RateLimit)
}
