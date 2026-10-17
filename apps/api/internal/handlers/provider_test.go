package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/models"
	"github.com/stretchr/testify/assert"
)

type fakeProviderStore struct {
	byID    map[string]*models.Provider
	updated []models.Provider
	deleted []string
}

func (f *fakeProviderStore) ListByUserID(context.Context, string) ([]models.Provider, error) {
	return nil, nil
}

func (f *fakeProviderStore) FindByID(_ context.Context, id string) (*models.Provider, error) {
	p, ok := f.byID[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	cp := *p
	return &cp, nil
}

func (f *fakeProviderStore) Create(context.Context, *models.Provider) error { return nil }

func (f *fakeProviderStore) Update(_ context.Context, p *models.Provider) error {
	f.updated = append(f.updated, *p)
	return nil
}

func (f *fakeProviderStore) Delete(_ context.Context, id, _ string) error {
	f.deleted = append(f.deleted, id)
	return nil
}

func providerRequest(h *ProviderHandler, user, method, id, body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("userId", user) })
	r.PUT("/api/providers/:id", h.Update)
	r.DELETE("/api/providers/:id", h.Delete)
	req := httptest.NewRequest(method, "/api/providers/"+id, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func newProviderTest() (*ProviderHandler, *fakeProviderStore) {
	store := &fakeProviderStore{byID: map[string]*models.Provider{
		"system": {ID: "system", UserID: "", Name: "OpenAI", BaseURL: "https://api.openai.com"},
		"mine":   {ID: "mine", UserID: "alice", Name: "Mine", BaseURL: "https://mine.example"},
		"bobs":   {ID: "bobs", UserID: "bob", Name: "Bob's", BaseURL: "https://bob.example"},
	}}
	return NewProviderHandler(store, nil), store
}

func TestSystemProvidersAreReadOnlyForUsers(t *testing.T) {
	h, store := newProviderTest()

	w := providerRequest(h, "alice", http.MethodPut, "system", `{"baseUrl":"https://attacker.example"}`)
	assert.Equal(t, http.StatusForbidden, w.Code)
	w = providerRequest(h, "alice", http.MethodDelete, "system", "")
	assert.Equal(t, http.StatusForbidden, w.Code)

	assert.Empty(t, store.updated)
	assert.Empty(t, store.deleted)
}

func TestProviderChangesRequireOwnership(t *testing.T) {
	// The fixture's base URLs are unresolvable; the outbound guard has its
	// own test (TestProviderBaseURLMustBePublic).
	t.Setenv("APP_ENV", "development")
	t.Setenv("ALLOW_INTERNAL_SSRF", "true")
	h, store := newProviderTest()

	assert.Equal(t, http.StatusNotFound, providerRequest(h, "alice", http.MethodPut, "bobs", `{"name":"x"}`).Code)
	assert.Equal(t, http.StatusNotFound, providerRequest(h, "alice", http.MethodDelete, "bobs", "").Code)
	assert.Empty(t, store.updated)
	assert.Empty(t, store.deleted)

	w := providerRequest(h, "alice", http.MethodPut, "mine", `{"name":"Renamed","userId":"bob","id":"bobs"}`)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	if assert.Len(t, store.updated, 1) {
		got := store.updated[0]
		assert.Equal(t, "Renamed", got.Name)
		assert.Equal(t, "mine", got.ID, "the body cannot retarget another provider")
		assert.Equal(t, "alice", got.UserID, "the body cannot hand the provider to someone else")
	}

	assert.Equal(t, http.StatusNoContent, providerRequest(h, "alice", http.MethodDelete, "mine", "").Code)
	assert.Equal(t, []string{"mine"}, store.deleted)
}
