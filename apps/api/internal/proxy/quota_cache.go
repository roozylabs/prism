package proxy

import (
	"context"
	"sync"
	"time"

	"github.com/roozylabs/prism/internal/models"
	"golang.org/x/sync/singleflight"
)

// QuotaEvaluator evaluates a tenant's spend quota.
type QuotaEvaluator interface {
	EvaluateQuota(ctx context.Context, targetType, targetID string) (*models.QuotaCheckResult, error)
}

// DefaultQuotaCacheTTL matches the budget manager's spend cache: a tenant can
// overshoot its quota by at most this much traffic before requests are denied.
const DefaultQuotaCacheTTL = 10 * time.Second

// maxQuotaCacheEntries bounds the cache; expired entries are swept when it is
// reached, and the cache is reset if that is not enough.
const maxQuotaCacheEntries = 10000

// CachedQuotaEvaluator caches quota evaluations per target for a short TTL,
// so the gateway does not query spend from request_logs on every request.
// Concurrent misses for the same target share one evaluation.
type CachedQuotaEvaluator struct {
	src   QuotaEvaluator
	ttl   time.Duration
	now   func() time.Time
	group singleflight.Group

	mu      sync.Mutex
	entries map[string]quotaCacheEntry
}

type quotaCacheEntry struct {
	res     *models.QuotaCheckResult
	expires time.Time
}

func NewCachedQuotaEvaluator(src QuotaEvaluator, ttl time.Duration) *CachedQuotaEvaluator {
	return &CachedQuotaEvaluator{src: src, ttl: ttl, now: time.Now, entries: make(map[string]quotaCacheEntry)}
}

// EvaluateQuota returns the cached result for the target, evaluating it at
// most once per TTL. Errors are not cached. The returned result is shared
// between callers and must not be modified.
func (c *CachedQuotaEvaluator) EvaluateQuota(ctx context.Context, targetType, targetID string) (*models.QuotaCheckResult, error) {
	key := targetType + "/" + targetID

	c.mu.Lock()
	if e, ok := c.entries[key]; ok && c.now().Before(e.expires) {
		c.mu.Unlock()
		return e.res, nil
	}
	c.mu.Unlock()

	// One caller's cancellation must not fail the evaluation shared with others.
	evalCtx := context.WithoutCancel(ctx)
	v, err, _ := c.group.Do(key, func() (interface{}, error) {
		res, err := c.src.EvaluateQuota(evalCtx, targetType, targetID)
		if err != nil {
			return nil, err
		}
		c.mu.Lock()
		if len(c.entries) >= maxQuotaCacheEntries {
			c.sweepLocked()
		}
		c.entries[key] = quotaCacheEntry{res: res, expires: c.now().Add(c.ttl)}
		c.mu.Unlock()
		return res, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*models.QuotaCheckResult), nil
}

func (c *CachedQuotaEvaluator) sweepLocked() {
	now := c.now()
	for k, e := range c.entries {
		if !now.Before(e.expires) {
			delete(c.entries, k)
		}
	}
	if len(c.entries) >= maxQuotaCacheEntries {
		c.entries = make(map[string]quotaCacheEntry)
	}
}
