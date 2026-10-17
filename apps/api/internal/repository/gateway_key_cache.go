package repository

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/roozylabs/prism/internal/models"
)

const (
	// gatewayKeyCacheTTL bounds how long a deleted key keeps working on an
	// instance that did not handle the delete (the other colour of a
	// blue/green deploy); the instance that did drops it at once.
	gatewayKeyCacheTTL = 10 * time.Second
	// gatewayKeyNegativeTTL bounds how long an unknown key is refused without
	// a database lookup, so a flood of invalid keys stays off the database.
	gatewayKeyNegativeTTL = 5 * time.Second
	// maxNegativeEntries caps the memory random invalid keys can take.
	maxNegativeEntries = 10_000
)

type gwKeyCacheEntry struct {
	key       *models.GatewayAPIKey // nil for a key that does not exist
	expiresAt time.Time
}

// gatewayKeyFinder is what the cache needs from the repository.
type gatewayKeyFinder interface {
	FindByKeyHash(ctx context.Context, keyHash string) (*models.GatewayAPIKey, error)
	FindByKeyPrefix(ctx context.Context, keyPrefix string) (*models.GatewayAPIKey, error)
}

// GatewayKeyCache wraps GatewayKeyRepository with a short-TTL in-memory cache
// keyed by key hash, eliminating a per-request DB roundtrip on the gateway hot
// path. Keys that do not exist are cached briefly too.
type GatewayKeyCache struct {
	repo        gatewayKeyFinder
	cache       sync.Map
	ttl         time.Duration
	negativeTTL time.Duration
	negatives   atomic.Int64
	sweepMu     sync.Mutex
	lastSweep   time.Time
}

func NewGatewayKeyCache(repo gatewayKeyFinder) *GatewayKeyCache {
	return &GatewayKeyCache{
		repo:        repo,
		ttl:         gatewayKeyCacheTTL,
		negativeTTL: gatewayKeyNegativeTTL,
		lastSweep:   time.Now(),
	}
}

func (c *GatewayKeyCache) FindByKeyHash(ctx context.Context, keyHash string) (*models.GatewayAPIKey, error) {
	c.sweep()

	if v, ok := c.cache.Load(keyHash); ok {
		entry := v.(gwKeyCacheEntry)
		if time.Now().Before(entry.expiresAt) {
			if entry.key == nil {
				return nil, sql.ErrNoRows
			}
			return entry.key, nil
		}
		c.remove(keyHash, entry)
	}

	key, err := c.repo.FindByKeyHash(ctx, keyHash)
	if err != nil {
		// Only "no such key" is worth remembering; a database error is not
		// an answer about the key.
		if errors.Is(err, sql.ErrNoRows) && c.negatives.Load() < maxNegativeEntries {
			if _, loaded := c.cache.LoadOrStore(keyHash, gwKeyCacheEntry{expiresAt: time.Now().Add(c.negativeTTL)}); !loaded {
				c.negatives.Add(1)
			}
		}
		return nil, err
	}

	if prev, loaded := c.cache.Swap(keyHash, gwKeyCacheEntry{key: key, expiresAt: time.Now().Add(c.ttl)}); loaded && prev.(gwKeyCacheEntry).key == nil {
		c.negatives.Add(-1)
	}
	return key, nil
}

func (c *GatewayKeyCache) FindByKeyPrefix(ctx context.Context, keyPrefix string) (*models.GatewayAPIKey, error) {
	return c.repo.FindByKeyPrefix(ctx, keyPrefix)
}

// Invalidate forces a cache miss for the given hash on next lookup. Call it
// when the key is deleted or changed.
func (c *GatewayKeyCache) Invalidate(keyHash string) {
	if v, ok := c.cache.Load(keyHash); ok {
		c.remove(keyHash, v.(gwKeyCacheEntry))
	}
}

func (c *GatewayKeyCache) remove(keyHash string, entry gwKeyCacheEntry) {
	if _, loaded := c.cache.LoadAndDelete(keyHash); loaded && entry.key == nil {
		c.negatives.Add(-1)
	}
}

// sweep drops expired entries at most once per TTL. Without it, hashes that
// are never looked up again would stay in the map for the life of the
// process.
func (c *GatewayKeyCache) sweep() {
	c.sweepMu.Lock()
	if time.Since(c.lastSweep) < c.ttl {
		c.sweepMu.Unlock()
		return
	}
	c.lastSweep = time.Now()
	c.sweepMu.Unlock()

	now := time.Now()
	c.cache.Range(func(k, v any) bool {
		entry := v.(gwKeyCacheEntry)
		if now.After(entry.expiresAt) {
			c.remove(k.(string), entry)
		}
		return true
	})
}
