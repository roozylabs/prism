package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/models"
	prismredis "github.com/roozylabs/prism/internal/redis"
)

// RateLimiter decides whether one more request fits a key's window.
type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int) (prismredis.Decision, error)
}

// GatewayRateLimitMiddleware limits requests per gateway key per minute. The
// key's own rate_limit applies, or defaultLimit when the key has none. When
// the limiter cannot be reached the request goes through (see the Redis
// failure table in docs/INFRASTRUCTURE.md), and that is logged.
func GatewayRateLimitMiddleware(limiter RateLimiter, defaultLimit int) gin.HandlerFunc {
	return func(c *gin.Context) {
		gatewayKey := c.MustGet("gatewayKey").(*models.GatewayAPIKey)
		tc := GetTenantContext(c)

		limit := gatewayKey.RateLimit
		if limit <= 0 {
			limit = defaultLimit
		}
		if limit <= 0 {
			c.Next()
			return
		}

		key := fmt.Sprintf("tenant:%s:gateway:%s:rate_limit", tc.OrgID, gatewayKey.KeyHash)
		d, err := limiter.Allow(c.Request.Context(), key, limit)
		if err != nil {
			logRateLimitFailOpen(err)
			c.Next()
			return
		}

		remaining := int64(limit) - d.Count
		if remaining < 0 {
			remaining = 0
		}
		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))

		if !d.Allowed {
			c.Header("Retry-After", strconv.FormatInt(int64(d.RetryAfter/time.Second), 10))
			httputil.RespondError(c, http.StatusTooManyRequests, "Rate limit exceeded", nil, "RATE_LIMIT_EXCEEDED")
			return
		}

		c.Next()
	}
}

var (
	rateLimitLogMu   sync.Mutex
	rateLimitLogLast time.Time
)

// logRateLimitFailOpen reports a limiter failure at most once every 10
// seconds: during a Redis outage every gateway request would hit it.
func logRateLimitFailOpen(err error) {
	rateLimitLogMu.Lock()
	defer rateLimitLogMu.Unlock()
	if time.Since(rateLimitLogLast) < 10*time.Second {
		return
	}
	rateLimitLogLast = time.Now()
	log.Printf("gateway rate limit: limiter unavailable, letting requests through: %v", err)
}
