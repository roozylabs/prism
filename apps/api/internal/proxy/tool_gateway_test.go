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

type toolFinderMock struct {
	twb *models.ToolWithBackends
	err error
}

func (m *toolFinderMock) GetToolWithBackends(ctx context.Context, userID, toolName string) (*models.ToolWithBackends, error) {
	return m.twb, m.err
}

func TestToolGatewayExecuteSuccess(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("ALLOW_INTERNAL_SSRF", "true")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var args map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&args)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": "search results for " + args["query"].(string)})
	}))
	defer srv.Close()

	twb := &models.ToolWithBackends{
		Tool: models.Tool{ID: "t1", UserID: "u1", Name: "search_web", Enabled: true},
		Backends: []models.ToolBackend{
			{ID: "b1", ToolID: "t1", Name: "test", EndpointURL: srv.URL, TimeoutMs: 5000, Priority: 1, Enabled: true},
		},
	}

	gw := NewToolGateway(&toolFinderMock{twb: twb})
	result, err := gw.Execute(context.Background(), "u1", "search_web", map[string]interface{}{"query": "hello"}, "")
	require.NoError(t, err)
	assert.Equal(t, 200, result.StatusCode)
	assert.Equal(t, "test", result.Backend)
	assert.Equal(t, "search_web", result.Tool)
	assert.GreaterOrEqual(t, result.LatencyMs, 0)
}

func TestToolGatewayExecuteFailover(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("ALLOW_INTERNAL_SSRF", "true")
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(500)
			_, _ = w.Write([]byte("server error"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": "ok from fallback"})
	}))
	defer srv.Close()

	twb := &models.ToolWithBackends{
		Tool: models.Tool{ID: "t1", UserID: "u1", Name: "search_web", Enabled: true},
		Backends: []models.ToolBackend{
			{ID: "b1", ToolID: "t1", Name: "primary", EndpointURL: srv.URL, TimeoutMs: 5000, Priority: 1, Enabled: true},
			{ID: "b2", ToolID: "t1", Name: "fallback", EndpointURL: srv.URL, TimeoutMs: 5000, Priority: 2, Enabled: true},
		},
	}

	gw := NewToolGateway(&toolFinderMock{twb: twb})
	result, err := gw.Execute(context.Background(), "u1", "search_web", map[string]interface{}{"query": "test"}, "")
	require.NoError(t, err)
	assert.Equal(t, 200, result.StatusCode)
	assert.Equal(t, "fallback", result.Backend)
}

func TestToolGatewayExecuteAllFail(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("ALLOW_INTERNAL_SSRF", "true")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	twb := &models.ToolWithBackends{
		Tool: models.Tool{ID: "t1", UserID: "u1", Name: "search_web", Enabled: true},
		Backends: []models.ToolBackend{
			{ID: "b1", ToolID: "t1", Name: "only", EndpointURL: srv.URL, TimeoutMs: 5000, Priority: 1, Enabled: true},
		},
	}

	gw := NewToolGateway(&toolFinderMock{twb: twb})
	_, err := gw.Execute(context.Background(), "u1", "search_web", map[string]interface{}{}, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "all backends failed")
}

func TestToolGatewaySSRFBlocked(t *testing.T) {
	twb := &models.ToolWithBackends{
		Tool: models.Tool{ID: "t1", UserID: "u1", Name: "probe_local", Enabled: true},
		Backends: []models.ToolBackend{
			{ID: "b1", ToolID: "t1", Name: "metadata", EndpointURL: "http://169.254.169.254/latest/meta-data/", TimeoutMs: 5000, Priority: 1, Enabled: true},
		},
	}
	gw := NewToolGateway(&toolFinderMock{twb: twb})
	_, err := gw.Execute(context.Background(), "u1", "probe_local", map[string]interface{}{}, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ssrf validation failed")
}

func TestToolGatewayNoBackends(t *testing.T) {
	twb := &models.ToolWithBackends{
		Tool:     models.Tool{ID: "t1", UserID: "u1", Name: "search_web", Enabled: true},
		Backends: []models.ToolBackend{},
	}
	gw := NewToolGateway(&toolFinderMock{twb: twb})
	_, err := gw.Execute(context.Background(), "u1", "search_web", map[string]interface{}{}, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no enabled backends")
}

func TestToolGatewayToolDisabled(t *testing.T) {
	twb := &models.ToolWithBackends{
		Tool:     models.Tool{ID: "t1", UserID: "u1", Name: "search_web", Enabled: false},
		Backends: []models.ToolBackend{{ID: "b1", Name: "x", EndpointURL: "http://x", TimeoutMs: 1000}},
	}
	gw := NewToolGateway(&toolFinderMock{twb: twb})
	_, err := gw.Execute(context.Background(), "u1", "search_web", map[string]interface{}{}, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

func TestToolGatewayToolNotFound(t *testing.T) {
	gw := NewToolGateway(&toolFinderMock{err: nil, twb: nil})
	_, err := gw.Execute(context.Background(), "u1", "nonexistent", map[string]interface{}{}, "")
	assert.Error(t, err)
}
