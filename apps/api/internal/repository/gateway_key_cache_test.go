package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/roozylabs/prism/internal/models"
)

type fakeKeyFinder struct {
	keys  map[string]*models.GatewayAPIKey
	err   error
	calls int
}

func (f *fakeKeyFinder) FindByKeyHash(_ context.Context, keyHash string) (*models.GatewayAPIKey, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	if k, ok := f.keys[keyHash]; ok {
		return k, nil
	}
	return nil, sql.ErrNoRows
}

func (f *fakeKeyFinder) FindByKeyPrefix(context.Context, string) (*models.GatewayAPIKey, error) {
	return nil, sql.ErrNoRows
}

func newTestKeyCache(finder *fakeKeyFinder) *GatewayKeyCache {
	c := NewGatewayKeyCache(finder)
	c.ttl = 50 * time.Millisecond
	c.negativeTTL = 30 * time.Millisecond
	return c
}

func TestGatewayKeyCacheServesHitsFromMemory(t *testing.T) {
	f := &fakeKeyFinder{keys: map[string]*models.GatewayAPIKey{"h1": {ID: "k1", KeyHash: "h1"}}}
	c := newTestKeyCache(f)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		k, err := c.FindByKeyHash(ctx, "h1")
		if err != nil || k.ID != "k1" {
			t.Fatalf("lookup %d: %v, %v", i, k, err)
		}
	}
	if f.calls != 1 {
		t.Fatalf("repository called %d times, want 1", f.calls)
	}

	time.Sleep(60 * time.Millisecond)
	if _, err := c.FindByKeyHash(ctx, "h1"); err != nil {
		t.Fatal(err)
	}
	if f.calls != 2 {
		t.Fatalf("repository called %d times after the TTL, want 2", f.calls)
	}
}

func TestGatewayKeyCacheRemembersUnknownKeysBriefly(t *testing.T) {
	f := &fakeKeyFinder{keys: map[string]*models.GatewayAPIKey{}}
	c := newTestKeyCache(f)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if _, err := c.FindByKeyHash(ctx, "unknown"); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("lookup %d: err %v, want sql.ErrNoRows", i, err)
		}
	}
	if f.calls != 1 {
		t.Fatalf("repository called %d times for an unknown key, want 1", f.calls)
	}
	if n := c.negatives.Load(); n != 1 {
		t.Fatalf("negative entries %d, want 1", n)
	}

	// A key created meanwhile is found again once the negative entry expires.
	f.keys["unknown"] = &models.GatewayAPIKey{ID: "k2", KeyHash: "unknown"}
	time.Sleep(40 * time.Millisecond)
	k, err := c.FindByKeyHash(ctx, "unknown")
	if err != nil || k.ID != "k2" {
		t.Fatalf("after the negative TTL: %v, %v", k, err)
	}
	if n := c.negatives.Load(); n != 0 {
		t.Fatalf("negative entries %d after a hit replaced the entry, want 0", n)
	}
}

func TestGatewayKeyCacheDoesNotCacheDatabaseErrors(t *testing.T) {
	f := &fakeKeyFinder{err: errors.New("connection refused")}
	c := newTestKeyCache(f)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		if _, err := c.FindByKeyHash(ctx, "h1"); err == nil {
			t.Fatal("database error swallowed")
		}
	}
	if f.calls != 2 {
		t.Fatalf("repository called %d times, want every lookup to retry after an error", f.calls)
	}
	if n := c.negatives.Load(); n != 0 {
		t.Fatalf("negative entries %d, want none for a database error", n)
	}
}

func TestGatewayKeyCacheInvalidateForcesRefetch(t *testing.T) {
	f := &fakeKeyFinder{keys: map[string]*models.GatewayAPIKey{"h1": {ID: "k1", KeyHash: "h1"}}}
	c := newTestKeyCache(f)
	ctx := context.Background()

	if _, err := c.FindByKeyHash(ctx, "h1"); err != nil {
		t.Fatal(err)
	}
	delete(f.keys, "h1")
	c.Invalidate("h1")

	if _, err := c.FindByKeyHash(ctx, "h1"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted key still served: err %v", err)
	}
	if f.calls != 2 {
		t.Fatalf("repository called %d times, want a refetch after Invalidate", f.calls)
	}
}

func TestGatewayKeyCacheBoundsNegativeEntries(t *testing.T) {
	f := &fakeKeyFinder{keys: map[string]*models.GatewayAPIKey{}}
	c := newTestKeyCache(f)
	c.negatives.Store(maxNegativeEntries)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		_, _ = c.FindByKeyHash(ctx, "one-more")
	}
	if f.calls != 2 {
		t.Fatalf("repository called %d times, want 2: at the cap nothing more is cached", f.calls)
	}
	if n := c.negatives.Load(); n != maxNegativeEntries {
		t.Fatalf("negative entries %d, want unchanged at the cap", n)
	}
}

func TestGatewayKeyCacheSweepDropsExpiredEntries(t *testing.T) {
	f := &fakeKeyFinder{keys: map[string]*models.GatewayAPIKey{"h1": {ID: "k1", KeyHash: "h1"}}}
	c := newTestKeyCache(f)
	ctx := context.Background()

	_, _ = c.FindByKeyHash(ctx, "h1")
	_, _ = c.FindByKeyHash(ctx, "gone")
	time.Sleep(60 * time.Millisecond)
	c.sweep()

	for _, h := range []string{"h1", "gone"} {
		if _, ok := c.cache.Load(h); ok {
			t.Fatalf("expired entry %q survived the sweep", h)
		}
	}
	if n := c.negatives.Load(); n != 0 {
		t.Fatalf("negative entries %d after the sweep, want 0", n)
	}
}
