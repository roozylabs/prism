package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/models"
	"github.com/stretchr/testify/require"
)

// ownerFixture backs the DB-backed handler tests: users alice and bob, an
// organization/workspace/project for alice, and one enabled provider each.
type ownerFixture struct {
	db                 *sql.DB
	provAlice, provBob string
	tenant             models.TenantContext
}

func newOwnerFixture(t *testing.T) *ownerFixture {
	t.Helper()
	db := migratedDB(t)
	f := &ownerFixture{db: db, tenant: models.TenantContext{OrgID: "org_alice", WorkspaceID: "ws_alice", ProjectID: "proj_alice", Role: "owner"}}
	_, err := db.Exec(`
		INSERT INTO "user" (id, name, email) VALUES ('alice', 'alice', 'alice@example.com'), ('bob', 'bob', 'bob@example.com');
		INSERT INTO organizations (id, name, slug) VALUES ('org_alice', 'Alice Org', 'alice-org');
		INSERT INTO workspaces (id, org_id, name, slug) VALUES ('ws_alice', 'org_alice', 'Main', 'main');
		INSERT INTO projects (id, workspace_id, name, slug) VALUES ('proj_alice', 'ws_alice', 'Default', 'default');`)
	require.NoError(t, err)
	for _, p := range []struct {
		user, slug string
		id         *string
	}{{"alice", "alice-openai", &f.provAlice}, {"bob", "bob-openai", &f.provBob}} {
		require.NoError(t, db.QueryRow(`INSERT INTO providers (user_id, name, slug, base_url, type, enabled, routing_strategy)
			VALUES ($1, $2, $2, 'http://localhost', 'openai', true, 'round_robin') RETURNING id`, p.user, p.slug).Scan(p.id), p.slug)
	}
	return f
}

// asUser returns a router whose requests carry the session user (and
// alice's tenant context when tenant is set), with routes registered by
// register.
func asUser(user string, tenant *models.TenantContext, register func(r *gin.Engine)) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if user != "" {
			c.Set("userId", user)
		}
		if tenant != nil {
			c.Set(middleware.TenantContextKey, *tenant)
			c.Set("organization_id", tenant.OrgID)
			c.Set("workspace_id", tenant.WorkspaceID)
		}
		c.Next()
	})
	register(r)
	return r
}

func doJSON(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// decodeBody parses a JSON object response.
func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out), w.Body.String())
	return out
}

// decodeList parses a JSON array response.
func decodeList(t *testing.T, w *httptest.ResponseRecorder) []map[string]interface{} {
	t.Helper()
	var out []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out), w.Body.String())
	return out
}

func requireStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	require.Equal(t, want, w.Code, w.Body.String())
}
