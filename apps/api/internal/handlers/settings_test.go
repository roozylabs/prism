package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/models"
	"github.com/stretchr/testify/assert"
)

type fakeSettingStore struct {
	writes map[string]map[string]string
}

func (f *fakeSettingStore) ListForOrg(_ context.Context, orgID string) ([]models.Setting, error) {
	var out []models.Setting
	for k, v := range f.writes[orgID] {
		out = append(out, models.Setting{Key: k, Value: v, Scope: "organization"})
	}
	return out, nil
}

func (f *fakeSettingStore) SetForOrg(_ context.Context, orgID string, s map[string]string) error {
	if f.writes == nil {
		f.writes = map[string]map[string]string{}
	}
	f.writes[orgID] = s
	return nil
}

func settingsRequest(h *SettingsHandler, orgID, method, body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if orgID != "" {
			c.Set(middleware.TenantContextKey, models.TenantContext{OrgID: orgID})
		}
	})
	r.GET("/api/settings", h.List)
	r.PUT("/api/settings", h.Update)
	req := httptest.NewRequest(method, "/api/settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestSettingsUpdateWritesOnlyTheCallersOrganization(t *testing.T) {
	store := &fakeSettingStore{}
	h := NewSettingsHandler(store)

	w := settingsRequest(h, "org_a", http.MethodPut, `{"settings":{"max_retries":"5"}}`)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, map[string]map[string]string{"org_a": {"max_retries": "5"}}, store.writes)
}

func TestSettingsRequireOrganization(t *testing.T) {
	store := &fakeSettingStore{}
	h := NewSettingsHandler(store)

	assert.Equal(t, http.StatusForbidden, settingsRequest(h, "", http.MethodGet, "").Code)
	assert.Equal(t, http.StatusForbidden, settingsRequest(h, "", http.MethodPut, `{"settings":{"a":"b"}}`).Code)
	assert.Empty(t, store.writes)
}

func TestSettingsUpdateValidatesInput(t *testing.T) {
	h := NewSettingsHandler(&fakeSettingStore{})
	for name, body := range map[string]string{
		"old flat shape":  `{"max_retries":"5"}`,
		"empty":           `{"settings":{}}`,
		"uppercase key":   `{"settings":{"Max":"1"}}`,
		"key with spaces": `{"settings":{"a b":"1"}}`,
		"value too long":  `{"settings":{"a":"` + strings.Repeat("x", maxSettingValueLen+1) + `"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, http.StatusBadRequest, settingsRequest(h, "org_a", http.MethodPut, body).Code)
		})
	}
}
