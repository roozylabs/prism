package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/models"
	prismredis "github.com/roozylabs/prism/internal/redis"
)

type fakeLimiter struct {
	decision prismredis.Decision
	err      error
	keys     []string
	limits   []int
}

func (f *fakeLimiter) Allow(_ context.Context, key string, limit int) (prismredis.Decision, error) {
	f.keys = append(f.keys, key)
	f.limits = append(f.limits, limit)
	return f.decision, f.err
}

func serveRateLimited(t *testing.T, limiter RateLimiter, key *models.GatewayAPIKey) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("gatewayKey", key)
		c.Set(TenantContextKey, models.TenantContext{OrgID: "org1"})
	})
	r.Use(GatewayRateLimitMiddleware(limiter, 100))
	r.GET("/v1/models", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	return w
}

func TestGatewayRateLimitAllowsAndReportsRemaining(t *testing.T) {
	l := &fakeLimiter{decision: prismredis.Decision{Allowed: true, Count: 3}}
	w := serveRateLimited(t, l, &models.GatewayAPIKey{KeyHash: "h1"})

	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", w.Code)
	}
	if got := l.keys; len(got) != 1 || got[0] != "tenant:org1:gateway:h1:rate_limit" {
		t.Fatalf("limiter key %v", got)
	}
	if l.limits[0] != 100 {
		t.Fatalf("limit %d, want the default 100 for a key without one", l.limits[0])
	}
	if h := w.Header(); h.Get("X-RateLimit-Limit") != "100" || h.Get("X-RateLimit-Remaining") != "97" {
		t.Fatalf("headers limit=%q remaining=%q", h.Get("X-RateLimit-Limit"), h.Get("X-RateLimit-Remaining"))
	}
}

func TestGatewayRateLimitUsesTheKeysOwnLimit(t *testing.T) {
	l := &fakeLimiter{decision: prismredis.Decision{Allowed: true, Count: 1}}
	w := serveRateLimited(t, l, &models.GatewayAPIKey{KeyHash: "h1", RateLimit: 5})

	if l.limits[0] != 5 {
		t.Fatalf("limit %d, want the key's 5", l.limits[0])
	}
	if h := w.Header(); h.Get("X-RateLimit-Limit") != "5" || h.Get("X-RateLimit-Remaining") != "4" {
		t.Fatalf("headers limit=%q remaining=%q", h.Get("X-RateLimit-Limit"), h.Get("X-RateLimit-Remaining"))
	}
}

func TestGatewayRateLimitRefusesWithRetryAfter(t *testing.T) {
	l := &fakeLimiter{decision: prismredis.Decision{Allowed: false, Count: 5, RetryAfter: 7 * time.Second}}
	w := serveRateLimited(t, l, &models.GatewayAPIKey{KeyHash: "h1", RateLimit: 5})

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status %d, want 429", w.Code)
	}
	if h := w.Header(); h.Get("Retry-After") != "7" || h.Get("X-RateLimit-Remaining") != "0" {
		t.Fatalf("headers retry-after=%q remaining=%q", h.Get("Retry-After"), h.Get("X-RateLimit-Remaining"))
	}
	if !strings.Contains(w.Body.String(), "RATE_LIMIT_EXCEEDED") {
		t.Fatalf("body %s", w.Body.String())
	}
}

func TestGatewayRateLimitFailsOpenWhenLimiterErrors(t *testing.T) {
	l := &fakeLimiter{err: errors.New("redis down")}
	w := serveRateLimited(t, l, &models.GatewayAPIKey{KeyHash: "h1"})

	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200 when the limiter is unavailable", w.Code)
	}
	if w.Header().Get("X-RateLimit-Limit") != "" {
		t.Fatal("rate limit headers set although no decision was made")
	}
}
