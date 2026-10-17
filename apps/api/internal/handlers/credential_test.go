package handlers

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/roozylabs/prism/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const credentialTestEncKey = "0123456789abcdef0123456789abcdef"

func credentialRoutes(h *CredentialHandler) func(r *gin.Engine) {
	return func(r *gin.Engine) {
		r.GET("/api/providers/:id/credentials", h.List)
		r.POST("/api/providers/:id/credentials", h.Create)
		r.GET("/api/providers/:id/credentials/:credId", h.Get)
		r.PUT("/api/providers/:id/credentials/:credId", h.Update)
		r.DELETE("/api/providers/:id/credentials/:credId", h.Delete)
		r.POST("/api/providers/:id/credentials/:credId/reveal", h.Reveal)
		r.POST("/api/providers/:id/credentials/:credId/reset-cooldown", h.ResetCooldown)
	}
}

func TestCredentialHandlerStoresEncryptedKeysForOwnProviders(t *testing.T) {
	f := newOwnerFixture(t)
	creds := repository.NewCredentialRepository(f.db)
	h := NewCredentialHandler(creds, repository.NewProviderRepository(f.db), repository.NewGatewayKeyRepository(f.db), nil, nil, credentialTestEncKey)
	alice := asUser("alice", &f.tenant, credentialRoutes(h))
	bob := asUser("bob", nil, credentialRoutes(h))
	base := "/api/providers/" + f.provAlice + "/credentials"

	// Only the owner (or a shared provider) accepts credentials, and the
	// provider must be enabled.
	w := doJSON(bob, http.MethodPost, base, `{"name":"sneaky","apiKey":"sk-bob"}`)
	requireStatus(t, w, http.StatusForbidden)
	var disabled string
	require.NoError(t, f.db.QueryRow(`INSERT INTO providers (user_id, name, slug, base_url, type, enabled, routing_strategy)
		VALUES ('alice', 'off', 'alice-off', 'http://localhost', 'openai', false, 'round_robin') RETURNING id`).Scan(&disabled))
	requireStatus(t, doJSON(alice, http.MethodPost, "/api/providers/"+disabled+"/credentials", `{"name":"x","apiKey":"sk-x"}`), http.StatusForbidden)
	requireStatus(t, doJSON(alice, http.MethodPost, base, `{"name":"no key"}`), http.StatusBadRequest)
	requireStatus(t, doJSON(alice, http.MethodPost, base, `{"name":"oauth","authType":"gcp_user_oauth"}`), http.StatusBadRequest)

	w = doJSON(alice, http.MethodPost, base, `{"name":"ops@example.com","apiKey":"sk-live-abcdef123456","priority":2}`)
	requireStatus(t, w, http.StatusCreated)
	created := decodeBody(t, w)
	id, _ := created["id"].(string)
	require.NotEmpty(t, id)
	assert.NotContains(t, w.Body.String(), "sk-live-abcdef123456", "the raw key never comes back")
	assert.NotContains(t, w.Body.String(), `"encryptedKey"`)
	assert.Equal(t, "sk-live-", created["keyPrefix"])
	assert.Equal(t, utils.MaskAPIKey("sk-live-abcdef123456"), created["maskedKey"])
	assert.Equal(t, "api_key", created["authType"])
	assert.Equal(t, "active", created["status"])
	assert.Equal(t, 2.0, created["priority"])

	// What was stored decrypts back to the key.
	stored, err := creds.FindByID(t.Context(), id, "alice")
	require.NoError(t, err)
	plain, err := utils.DecryptAES256GCM(stored.EncryptedKey, credentialTestEncKey)
	require.NoError(t, err)
	assert.Equal(t, "sk-live-abcdef123456", plain)
	require.NotNil(t, stored.OrgID)
	assert.Equal(t, "org_alice", *stored.OrgID)

	// Listing masks e-mail-like names; reveal returns the real one.
	w = doJSON(alice, http.MethodGet, base, "")
	requireStatus(t, w, http.StatusOK)
	page := decodeBody(t, w)
	assert.Equal(t, 1.0, page["total"])
	rows, _ := page["data"].([]interface{})
	require.Len(t, rows, 1)
	assert.Equal(t, "op***s@example.com", rows[0].(map[string]interface{})["name"])
	w = doJSON(alice, http.MethodPost, base+"/"+id+"/reveal", "")
	requireStatus(t, w, http.StatusOK)
	assert.Equal(t, "ops@example.com", decodeBody(t, w)["name"])

	// Bob sees nothing of it.
	w = doJSON(bob, http.MethodGet, base, "")
	requireStatus(t, w, http.StatusOK)
	assert.Equal(t, 0.0, decodeBody(t, w)["total"])
	requireStatus(t, doJSON(bob, http.MethodGet, base+"/"+id, ""), http.StatusNotFound)
	requireStatus(t, doJSON(bob, http.MethodPost, base+"/"+id+"/reveal", ""), http.StatusNotFound)
	requireStatus(t, doJSON(bob, http.MethodPut, base+"/"+id, `{"name":"stolen"}`), http.StatusNotFound)
	requireStatus(t, doJSON(bob, http.MethodDelete, base+"/"+id, ""), http.StatusNotFound)

	// Rotating the key re-encrypts and re-masks it.
	w = doJSON(alice, http.MethodPut, base+"/"+id, `{"apiKey":"sk-rotated-9876","status":"disabled"}`)
	requireStatus(t, w, http.StatusOK)
	updated := decodeBody(t, w)
	assert.Equal(t, "sk-rotat", updated["keyPrefix"])
	assert.Equal(t, "sk-rotat••••9876", updated["maskedKey"])
	assert.Equal(t, false, updated["enabled"])
	stored, _ = creds.FindByID(t.Context(), id, "alice")
	plain, _ = utils.DecryptAES256GCM(stored.EncryptedKey, credentialTestEncKey)
	assert.Equal(t, "sk-rotated-9876", plain)

	w = doJSON(alice, http.MethodPost, base+"/"+id+"/reset-cooldown", "")
	requireStatus(t, w, http.StatusOK)

	// The last active credential of a provider with live gateway keys stays.
	_, err = f.db.Exec(`UPDATE credentials SET enabled = true, status = 'active' WHERE id = $1`, id)
	require.NoError(t, err)
	_, err = f.db.Exec(`INSERT INTO gateway_api_keys (user_id, provider_id, name, key_hash, key_prefix, enabled)
		VALUES ('alice', $1, 'bound', 'hash-bound', 'prism_sk_bnd', true)`, f.provAlice)
	require.NoError(t, err)
	w = doJSON(alice, http.MethodDelete, base+"/"+id, "")
	requireStatus(t, w, http.StatusConflict)
	assert.Contains(t, w.Body.String(), "ACTIVE_KEYS_EXIST")
	_, err = f.db.Exec(`UPDATE gateway_api_keys SET enabled = false WHERE key_hash = 'hash-bound'`)
	require.NoError(t, err)
	requireStatus(t, doJSON(alice, http.MethodDelete, base+"/"+id, ""), http.StatusNoContent)
	requireStatus(t, doJSON(alice, http.MethodGet, base+"/"+id, ""), http.StatusNotFound)
}
