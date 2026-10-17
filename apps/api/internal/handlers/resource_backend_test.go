package handlers

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func resourceCreate(h *ResourceHandler, body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("userId", "alice") })
	r.POST("/api/resources", h.Create)
	req := httptest.NewRequest(http.MethodPost, "/api/resources", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func strptr(s string) *string { return &s }

// Backends are checked before anything is stored, so the handler needs no
// repositories for the refusals.
func TestResourceBackendsAreValidatedBeforeStorage(t *testing.T) {
	t.Setenv("ALLOW_INTERNAL_SSRF", "")
	t.Setenv("APP_ENV", "production")
	h := NewResourceHandler(nil, nil, nil, "")

	// Postgres backends are off by default: not even a public host is accepted.
	w := resourceCreate(h, `{"name":"probe","backends":[{"name":"pg","backendType":"postgres","connectionString":"postgres://u:p@example.com:5432/db","sqlQuery":"select 1"}]}`)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "RESOURCE_POSTGRES_DISABLED")

	// Once enabled, the host must still be public.
	h.SetPostgresBackends(true)
	w = resourceCreate(h, `{"name":"probe","backends":[{"name":"pg","backendType":"postgres","connectionString":"postgres://postgres:postgres@localhost:5432/prism","sqlQuery":"select 1"}]}`)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "RESOURCE_BACKEND_BLOCKED")

	// REST and GraphQL endpoints go through the outbound guard too.
	w = resourceCreate(h, `{"name":"probe","backends":[{"name":"api","backendType":"rest","endpointUrl":"http://api:8080/api/gateway-keys"}]}`)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "RESOURCE_BACKEND_BLOCKED")
	w = resourceCreate(h, `{"name":"probe","backends":[{"name":"api","backendType":"graphql"}]}`)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

func TestValidateBackendDecisions(t *testing.T) {
	t.Setenv("ALLOW_INTERNAL_SSRF", "")
	t.Setenv("APP_ENV", "production")
	h := NewResourceHandler(nil, nil, nil, "")

	err := h.validateBackend(CreateResourceBackendRequest{Name: "pg", BackendType: "postgres", ConnectionString: strptr("postgres://u:p@example.com/db")})
	assert.True(t, errors.Is(err, errPostgresBackendsDisabled), "disabled: %v", err)

	h.SetPostgresBackends(true)
	assert.Error(t, h.validateBackend(CreateResourceBackendRequest{Name: "pg", BackendType: "postgres"}), "missing connection string")
	assert.Error(t, h.validateBackend(CreateResourceBackendRequest{Name: "pg", BackendType: "Postgres", ConnectionString: strptr("postgres://u:p@10.1.2.3/db")}), "private host")
	assert.Error(t, h.validateBackend(CreateResourceBackendRequest{Name: "r", BackendType: "rest", EndpointURL: strptr("http://127.0.0.1:9/x")}), "loopback endpoint")

	if _, err := net.LookupIP("example.com"); err != nil {
		t.Skipf("no DNS: %v", err)
	}
	assert.NoError(t, h.validateBackend(CreateResourceBackendRequest{Name: "pg", BackendType: "postgres", ConnectionString: strptr("postgres://u:p@example.com:5432/db?sslmode=require")}))
	assert.NoError(t, h.validateBackend(CreateResourceBackendRequest{Name: "r", BackendType: "rest", EndpointURL: strptr("https://example.com/api")}))
}
