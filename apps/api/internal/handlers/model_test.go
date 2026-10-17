package handlers

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func modelRoutes(h *ModelHandler) func(r *gin.Engine) {
	return func(r *gin.Engine) {
		r.GET("/api/providers/:id/models", h.List)
		r.POST("/api/providers/:id/models", h.Create)
		r.GET("/api/providers/:id/models/:modelId", h.Get)
		r.PUT("/api/providers/:id/models/:modelId", h.Update)
		r.PATCH("/api/providers/:id/models/:modelId/capabilities", h.UpdateCapabilities)
		r.DELETE("/api/providers/:id/models/:modelId", h.Delete)
	}
}

func TestModelHandlerCRUDAndDeleteGuards(t *testing.T) {
	f := newOwnerFixture(t)
	h := NewModelHandler(repository.NewModelRepository(f.db), repository.NewProviderRepository(f.db), repository.NewGatewayKeyRepository(f.db), nil)
	alice := asUser("alice", nil, modelRoutes(h))
	base := "/api/providers/" + f.provAlice + "/models"

	requireStatus(t, doJSON(alice, http.MethodPost, base, `{not json`), http.StatusBadRequest)

	w := doJSON(alice, http.MethodPost, base, `{"name":"GPT X","slug":"gpt-x","displayName":"GPT X","enabled":true,"inputPricePer1M":2.5}`)
	requireStatus(t, w, http.StatusCreated)
	created := decodeBody(t, w)
	id, _ := created["id"].(string)
	require.NotEmpty(t, id)
	assert.Equal(t, f.provAlice, created["providerId"], "the provider comes from the path")
	assert.Equal(t, 2.5, created["inputPricePer1M"])

	requireStatus(t, doJSON(alice, http.MethodPost, base, `{"name":"Other","slug":"other","displayName":"Other","enabled":true}`), http.StatusCreated)

	w = doJSON(alice, http.MethodGet, base+"?search=gpt&limit=10", "")
	requireStatus(t, w, http.StatusOK)
	page := decodeBody(t, w)
	assert.Equal(t, 1.0, page["total"])
	w = doJSON(alice, http.MethodGet, base+"?limit=1&page=2", "")
	page = decodeBody(t, w)
	assert.Equal(t, 2.0, page["total"])
	assert.Equal(t, 2.0, page["totalPages"])
	assert.Len(t, page["data"], 1)

	w = doJSON(alice, http.MethodPatch, base+"/"+id+"/capabilities", `{"contextWindow":128000,"supportsTools":true,"qualityScore":0.9}`)
	requireStatus(t, w, http.StatusOK)
	caps := decodeBody(t, w)
	assert.Equal(t, 128000.0, caps["contextWindow"])
	assert.Equal(t, true, caps["supportsTools"])
	assert.Equal(t, 0.9, caps["qualityScore"])
	assert.Equal(t, 2.5, caps["inputPricePer1M"], "untouched capability values survive")

	w = doJSON(alice, http.MethodPut, base+"/"+id, `{"displayName":"GPT X (renamed)","enabled":false}`)
	requireStatus(t, w, http.StatusOK)
	w = doJSON(alice, http.MethodGet, base+"/"+id, "")
	requireStatus(t, w, http.StatusOK)
	got := decodeBody(t, w)
	assert.Equal(t, "GPT X (renamed)", got["displayName"])
	assert.Equal(t, false, got["enabled"])
	assert.Equal(t, "gpt-x", got["slug"], "fields missing from the body keep their values")

	// A gateway key that allows the model blocks deletion until it is disabled.
	_, err := f.db.Exec(`INSERT INTO gateway_api_keys (user_id, name, key_hash, key_prefix, enabled, allowed_models)
		VALUES ('alice', 'pinned', 'hash-pinned', 'prism_sk_pin', true, '{gpt-x}')`)
	require.NoError(t, err)
	w = doJSON(alice, http.MethodDelete, base+"/"+id, "")
	requireStatus(t, w, http.StatusConflict)
	assert.Contains(t, w.Body.String(), "MODEL_IN_USE_BY_KEYS")
	_, err = f.db.Exec(`UPDATE gateway_api_keys SET enabled = false WHERE key_hash = 'hash-pinned'`)
	require.NoError(t, err)
	requireStatus(t, doJSON(alice, http.MethodDelete, base+"/"+id, ""), http.StatusNoContent)
	requireStatus(t, doJSON(alice, http.MethodGet, base+"/"+id, ""), http.StatusNotFound)
	requireStatus(t, doJSON(alice, http.MethodDelete, base+"/"+id, ""), http.StatusNotFound)
}
