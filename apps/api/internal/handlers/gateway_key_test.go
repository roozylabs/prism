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

func gatewayKeyRoutes(h *GatewayKeyHandler) func(r *gin.Engine) {
	return func(r *gin.Engine) {
		r.GET("/api/gateway-keys", h.List)
		r.POST("/api/gateway-keys", h.Create)
		r.DELETE("/api/gateway-keys/:id", h.Delete)
	}
}

func TestGatewayKeyHandlerIssuesListsAndRevokesKeys(t *testing.T) {
	f := newOwnerFixture(t)
	keys := repository.NewGatewayKeyRepository(f.db)
	creds := repository.NewCredentialRepository(f.db)
	h := NewGatewayKeyHandler(keys, creds, nil)
	alice := asUser("alice", &f.tenant, gatewayKeyRoutes(h))
	bob := asUser("bob", nil, gatewayKeyRoutes(h))

	requireStatus(t, doJSON(alice, http.MethodPost, "/api/gateway-keys", `{"rateLimit":5}`), http.StatusBadRequest)

	// A provider-bound key needs an active credential on that provider.
	w := doJSON(alice, http.MethodPost, "/api/gateway-keys", `{"name":"bound","providerId":"`+f.provAlice+`"}`)
	requireStatus(t, w, http.StatusBadRequest)
	assert.Contains(t, w.Body.String(), "PROVIDER_NO_CREDENTIALS")

	_, err := f.db.Exec(`INSERT INTO credentials (provider_id, user_id, name, encrypted_key, key_prefix, enabled, status)
		VALUES ($1, 'alice', 'main', 'enc', 'sk-', true, 'active')`, f.provAlice)
	require.NoError(t, err)

	w = doJSON(alice, http.MethodPost, "/api/gateway-keys", `{"name":"bound","providerId":"`+f.provAlice+`","allowedModels":["gpt-4o"],"expiresInDays":7}`)
	requireStatus(t, w, http.StatusCreated)
	created := decodeBody(t, w)
	raw, _ := created["key"].(string)
	assert.True(t, strings.HasPrefix(raw, utils.GatewayKeyPrefix), "secret %q", raw)
	assert.NotContains(t, w.Body.String(), `"keyHash"`, "the hash never leaves the server")
	assert.Equal(t, f.provAlice, created["providerId"])
	assert.Equal(t, 100.0, created["rateLimit"], "default rate limit")
	assert.Equal(t, "org_alice", created["orgId"])
	assert.NotEmpty(t, created["expiresAt"])
	assert.Equal(t, []interface{}{"gpt-4o"}, created["allowedModels"])
	prefix, _ := created["keyPrefix"].(string)
	assert.True(t, strings.HasPrefix(raw, strings.TrimSuffix(prefix, "...")), "prefix %q of %q", prefix, raw)

	// The stored hash matches the secret and only the secret unlocks it.
	stored, err := keys.FindByKeyHash(t.Context(), utils.HashSHA256(raw))
	require.NoError(t, err)
	assert.Equal(t, created["id"], stored.ID)

	// "global" means unbound.
	w = doJSON(alice, http.MethodPost, "/api/gateway-keys", `{"name":"global","providerId":"global","rateLimit":7}`)
	requireStatus(t, w, http.StatusCreated)
	unbound := decodeBody(t, w)
	assert.Nil(t, unbound["providerId"])
	assert.Equal(t, 7.0, unbound["rateLimit"])
	requireStatus(t, doJSON(alice, http.MethodPost, "/api/gateway-keys", `{"name":"ci"}`), http.StatusCreated)

	w = doJSON(alice, http.MethodGet, "/api/gateway-keys?limit=2&page=2", "")
	requireStatus(t, w, http.StatusOK)
	page := decodeBody(t, w)
	assert.Equal(t, 3.0, page["total"])
	assert.Equal(t, 2.0, page["totalPages"])
	assert.Len(t, page["data"], 1)
	assert.NotContains(t, w.Body.String(), `"key":`, "listing never includes secrets")

	w = doJSON(alice, http.MethodGet, "/api/gateway-keys?search=glob", "")
	assert.Equal(t, 1.0, decodeBody(t, w)["total"])

	w = doJSON(bob, http.MethodGet, "/api/gateway-keys", "")
	requireStatus(t, w, http.StatusOK)
	assert.Equal(t, 0.0, decodeBody(t, w)["total"])

	id, _ := created["id"].(string)
	requireStatus(t, doJSON(bob, http.MethodDelete, "/api/gateway-keys/"+id, ""), http.StatusNotFound)
	requireStatus(t, doJSON(alice, http.MethodDelete, "/api/gateway-keys/"+id, ""), http.StatusNoContent)
	requireStatus(t, doJSON(alice, http.MethodDelete, "/api/gateway-keys/"+id, ""), http.StatusNotFound)
	w = doJSON(alice, http.MethodGet, "/api/gateway-keys", "")
	assert.Equal(t, 2.0, decodeBody(t, w)["total"])
}
