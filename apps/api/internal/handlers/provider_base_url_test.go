package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func providerCreate(h *ProviderHandler, body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("userId", "alice") })
	r.POST("/api/providers", h.Create)
	req := httptest.NewRequest(http.MethodPost, "/api/providers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// Gateway traffic goes to the provider's base URL, so it must not point at
// the API, the database, Redis or a cloud metadata service.
func TestProviderBaseURLMustBePublic(t *testing.T) {
	t.Setenv("ALLOW_INTERNAL_SSRF", "")
	t.Setenv("APP_ENV", "production")
	h, store := newProviderTest()

	for _, baseURL := range []string{
		"http://api:8080",        // compose service name
		"http://127.0.0.1:8080",  // loopback
		"http://localhost:6379",  // local alias
		"http://169.254.169.254", // metadata
		"http://10.0.0.5:5432",   // private
		"ftp://example.com",      // wrong scheme
		"",                       // missing
	} {
		w := providerCreate(h, `{"name":"x","slug":"x-`+strings.Map(func(r rune) rune {
			if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
				return r
			}
			return '-'
		}, baseURL)+`","baseUrl":"`+baseURL+`","type":"openai"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code, "%q: %s", baseURL, w.Body.String())
		assert.Contains(t, w.Body.String(), "PROVIDER_BASE_URL_BLOCKED", baseURL)
	}
	assert.Empty(t, store.updated)

	// Updates go through the same check.
	w := providerRequest(h, "alice", http.MethodPut, "mine", `{"baseUrl":"http://127.0.0.1:8080"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Empty(t, store.updated)
}
