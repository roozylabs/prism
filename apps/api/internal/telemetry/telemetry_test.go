package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func scrapeMetrics(t *testing.T) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/metrics", PrometheusHandler())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, w.Code)
	return w.Body.String()
}

// Without an OTLP collector the gateway's own instruments must still reach
// /metrics; before, they were no-ops unless OTEL_ENABLED was set.
func TestMetricsAreServedWithoutOTLP(t *testing.T) {
	t.Setenv("OTEL_ENABLED", "")
	ctx := context.Background()
	shutdown, err := InitOTel(ctx)
	require.NoError(t, err)
	defer func() { _ = shutdown(ctx) }()
	// A second call (another test, a retry) must not register the exporter
	// twice and break the endpoint.
	_, err = InitOTel(ctx)
	require.NoError(t, err)

	RecordRequestMetrics(ctx, "gpt-test", "openai", "200", "org-test", 0.42, 10, 20, 0.001)
	RecordTTFT(ctx, "gpt-test", "openai", 0.3)

	body := scrapeMetrics(t)
	assert.Contains(t, body, "prism_requests_total{")
	assert.Contains(t, body, `model="gpt-test"`)
	assert.Contains(t, body, "prism_token_usage_total{")
	assert.Contains(t, body, "prism_cost_usd_total{", "cost counter must keep its name")
	assert.NotContains(t, body, "prism_cost_usd_USD_total", "unit suffix must not be appended")
	assert.NotContains(t, body, "otel_scope_name=", "scope labels bloat every series")

	// Seconds-scale histograms: the default buckets start at 5 and would put
	// every request in the first bucket.
	assert.Contains(t, body, `prism_request_duration_seconds_bucket{`)
	assert.True(t, strings.Contains(body, `le="0.5"`) && strings.Contains(body, `le="1"`), "second-scale buckets missing:\n%s", body)
	assert.Contains(t, body, "prism_ttft_seconds_bucket{")
}

func TestPrometheusHandlerServesRuntimeMetrics(t *testing.T) {
	body := scrapeMetrics(t)
	assert.Contains(t, body, "go_goroutines")
}
