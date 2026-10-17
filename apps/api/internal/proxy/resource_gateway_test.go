package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/roozylabs/prism/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type resourceFinderMock struct {
	rwb *models.ResourceWithBackends
	err error
}

func (m *resourceFinderMock) GetResourceWithBackends(ctx context.Context, userID, resourceName string) (*models.ResourceWithBackends, error) {
	return m.rwb, m.err
}

func TestResourceGatewayRestSuccess(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("ALLOW_INTERNAL_SSRF", "true")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"name": "test", "id": "123"})
	}))
	defer srv.Close()
	ep := srv.URL
	finder := &resourceFinderMock{rwb: &models.ResourceWithBackends{
		Resource: models.Resource{ID: "r1", UserID: "u1", Name: "get_customer", Enabled: true},
		Backends: []models.ResourceBackend{{
			ID: "rb1", ResourceID: "r1", Name: "api", BackendType: "rest",
			EndpointURL: &ep, HTTPMethod: "GET", TimeoutMs: 5000, Priority: 1, Enabled: true,
		}},
	}}
	gw := NewResourceGateway(finder)
	result, err := gw.Execute(context.Background(), "u1", "get_customer", map[string]interface{}{"id": "123"}, "")
	require.NoError(t, err)
	assert.Equal(t, 200, result.StatusCode)
	assert.Equal(t, "get_customer", result.Resource)
}

func TestResourceGatewayGraphqlSuccess(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("ALLOW_INTERNAL_SSRF", "true")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := json.Marshal(r.Header.Get("Content-Type"))
		_ = body
		var payload map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		assert.Contains(t, payload, "query")
		assert.Contains(t, payload, "variables")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": []map[string]interface{}{{"name": "A"}}})
	}))
	defer srv.Close()
	query := "{ customers { name } }"
	ep := srv.URL
	finder := &resourceFinderMock{rwb: &models.ResourceWithBackends{
		Resource: models.Resource{ID: "r2", UserID: "u1", Name: "list_customers", Enabled: true},
		Backends: []models.ResourceBackend{{
			ID: "rb2", ResourceID: "r2", Name: "gql", BackendType: "graphql",
			EndpointURL: &ep, QueryTemplate: &query, TimeoutMs: 5000, Priority: 1, Enabled: true,
		}},
	}}
	gw := NewResourceGateway(finder)
	result, err := gw.Execute(context.Background(), "u1", "list_customers", map[string]interface{}{"limit": 10}, "")
	require.NoError(t, err)
	assert.Equal(t, 200, result.StatusCode)
	assert.GreaterOrEqual(t, result.RowCount, 1)
}

func TestResourceGatewayFailover(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("ALLOW_INTERNAL_SSRF", "true")
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": "ok"})
	}))
	defer srv.Close()
	ep := srv.URL
	finder := &resourceFinderMock{rwb: &models.ResourceWithBackends{
		Resource: models.Resource{ID: "r3", UserID: "u1", Name: "fetch", Enabled: true},
		Backends: []models.ResourceBackend{
			{ID: "rb3", ResourceID: "r3", Name: "primary", BackendType: "rest",
				EndpointURL: &ep, HTTPMethod: "POST", TimeoutMs: 5000, Priority: 1, Enabled: true},
			{ID: "rb4", ResourceID: "r3", Name: "fallback", BackendType: "rest",
				EndpointURL: &ep, HTTPMethod: "POST", TimeoutMs: 5000, Priority: 2, Enabled: true},
		},
	}}
	gw := NewResourceGateway(finder)
	result, err := gw.Execute(context.Background(), "u1", "fetch", map[string]interface{}{}, "")
	require.NoError(t, err)
	assert.Equal(t, 200, result.StatusCode)
	assert.Equal(t, "fallback", result.Backend)
}

func TestResourceGatewaySSRFBlocked(t *testing.T) {
	ep := "http://127.0.0.1:8080/internal/metrics"
	finder := &resourceFinderMock{rwb: &models.ResourceWithBackends{
		Resource: models.Resource{ID: "r_ssrf", UserID: "u1", Name: "probe_local", Enabled: true},
		Backends: []models.ResourceBackend{
			{ID: "rb_ssrf", ResourceID: "r_ssrf", Name: "internal_ip", BackendType: "rest",
				EndpointURL: &ep, HTTPMethod: "GET", TimeoutMs: 5000, Priority: 1, Enabled: true},
		},
	}}
	gw := NewResourceGateway(finder)
	_, err := gw.Execute(context.Background(), "u1", "probe_local", map[string]interface{}{}, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ssrf validation failed")
}

func TestBindParams(t *testing.T) {
	args := map[string]interface{}{"name": "Alice", "age": 30}
	names := []string{"age", "name"}
	result, err := bindParams(names, args)
	require.NoError(t, err)
	assert.Equal(t, 30, result[0])
	assert.Equal(t, "Alice", result[1])

	_, err = bindParams([]string{"missing"}, args)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing")
}

func TestResourceGatewayNoBackends(t *testing.T) {
	finder := &resourceFinderMock{rwb: &models.ResourceWithBackends{
		Resource: models.Resource{ID: "r4", UserID: "u1", Name: "x", Enabled: true},
		Backends: []models.ResourceBackend{},
	}}
	gw := NewResourceGateway(finder)
	_, err := gw.Execute(context.Background(), "u1", "x", map[string]interface{}{}, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no enabled backends")
}

func TestResourceGatewayDisabled(t *testing.T) {
	finder := &resourceFinderMock{rwb: &models.ResourceWithBackends{
		Resource: models.Resource{ID: "r5", UserID: "u1", Name: "x", Enabled: false},
		Backends: []models.ResourceBackend{{ID: "rb", Name: "n", EndpointURL: &[]string{"http://x"}[0], TimeoutMs: 1000}},
	}}
	gw := NewResourceGateway(finder)
	_, err := gw.Execute(context.Background(), "u1", "x", map[string]interface{}{}, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

func TestCountRows(t *testing.T) {
	assert.Equal(t, 0, countRows([]interface{}{}))
	assert.Equal(t, 2, countRows([]interface{}{"a", "b"}))
	assert.Equal(t, 2, countRows(map[string]interface{}{"data": []interface{}{"a", "b"}}))
	assert.Equal(t, 1, countRows(map[string]interface{}{"result": "ok"}))
	assert.Equal(t, 0, countRows(""))
}
