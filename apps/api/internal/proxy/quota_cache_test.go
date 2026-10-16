package proxy

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/roozylabs/prism/internal/models"
)

type fakeQuotaSource struct {
	calls   atomic.Int32
	allowed bool
	err     error
	block   chan struct{}
}

func (f *fakeQuotaSource) EvaluateQuota(_ context.Context, targetType, targetID string) (*models.QuotaCheckResult, error) {
	f.calls.Add(1)
	if f.block != nil {
		<-f.block
	}
	if f.err != nil {
		return nil, f.err
	}
	return &models.QuotaCheckResult{Allowed: f.allowed, TargetType: targetType, TargetID: targetID}, nil
}

func newTestQuotaCache(src QuotaEvaluator, ttl time.Duration) (*CachedQuotaEvaluator, *time.Time) {
	c := NewCachedQuotaEvaluator(src, ttl)
	now := time.Unix(1_700_000_000, 0)
	c.now = func() time.Time { return now }
	return c, &now
}

func TestCachedQuotaEvaluator_HitWithinTTL(t *testing.T) {
	src := &fakeQuotaSource{allowed: true}
	c, _ := newTestQuotaCache(src, 10*time.Second)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		res, err := c.EvaluateQuota(ctx, "organization", "org_a")
		if err != nil || !res.Allowed {
			t.Fatalf("unexpected result %+v, %v", res, err)
		}
	}
	if got := src.calls.Load(); got != 1 {
		t.Errorf("expected 1 evaluation within the TTL, got %d", got)
	}
}

func TestCachedQuotaEvaluator_ExpiresAndKeysByTarget(t *testing.T) {
	src := &fakeQuotaSource{allowed: false}
	c, now := newTestQuotaCache(src, 10*time.Second)
	ctx := context.Background()

	res, _ := c.EvaluateQuota(ctx, "organization", "org_a")
	if res.Allowed {
		t.Fatal("a denial should be returned (and cached) as is")
	}
	_, _ = c.EvaluateQuota(ctx, "organization", "org_b")
	if got := src.calls.Load(); got != 2 {
		t.Fatalf("each org needs its own evaluation, got %d calls", got)
	}

	*now = now.Add(11 * time.Second)
	_, _ = c.EvaluateQuota(ctx, "organization", "org_a")
	if got := src.calls.Load(); got != 3 {
		t.Errorf("expired entry should be re-evaluated, got %d calls", got)
	}
}

func TestCachedQuotaEvaluator_ErrorsAreNotCached(t *testing.T) {
	src := &fakeQuotaSource{err: errors.New("db down")}
	c, _ := newTestQuotaCache(src, 10*time.Second)
	ctx := context.Background()

	if _, err := c.EvaluateQuota(ctx, "organization", "org_a"); err == nil {
		t.Fatal("expected the source error")
	}
	src.err = nil
	src.allowed = true
	res, err := c.EvaluateQuota(ctx, "organization", "org_a")
	if err != nil || !res.Allowed || src.calls.Load() != 2 {
		t.Errorf("an error must not be cached: res %+v, err %v, calls %d", res, err, src.calls.Load())
	}
}

func TestCachedQuotaEvaluator_ConcurrentMissesShareOneEvaluation(t *testing.T) {
	src := &fakeQuotaSource{allowed: true, block: make(chan struct{})}
	c, _ := newTestQuotaCache(src, 10*time.Second)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if res, err := c.EvaluateQuota(context.Background(), "organization", "org_a"); err != nil || !res.Allowed {
				t.Errorf("unexpected result %+v, %v", res, err)
			}
		}()
	}
	// Let every goroutine reach the shared evaluation before releasing it.
	for src.calls.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	close(src.block)
	wg.Wait()

	if got := src.calls.Load(); got != 1 {
		t.Errorf("concurrent misses should share one evaluation, got %d", got)
	}
}

func TestCachedQuotaEvaluator_SweepDropsExpiredEntries(t *testing.T) {
	c, now := newTestQuotaCache(&fakeQuotaSource{allowed: true}, 10*time.Second)
	c.entries["organization/old"] = quotaCacheEntry{expires: now.Add(-time.Second)}
	c.entries["organization/live"] = quotaCacheEntry{expires: now.Add(time.Second)}

	c.sweepLocked()

	if _, ok := c.entries["organization/old"]; ok {
		t.Error("expired entry should be swept")
	}
	if _, ok := c.entries["organization/live"]; !ok {
		t.Error("live entry should be kept")
	}
}

type stuckQuotaSource struct{ sawDeadline chan bool }

func (s stuckQuotaSource) EvaluateQuota(ctx context.Context, _, _ string) (*models.QuotaCheckResult, error) {
	_, ok := ctx.Deadline()
	s.sawDeadline <- ok
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestCachedQuotaEvaluator_StuckEvaluationDoesNotHoldCallers(t *testing.T) {
	src := stuckQuotaSource{sawDeadline: make(chan bool, 1)}
	c := NewCachedQuotaEvaluator(src, 10*time.Second)
	c.timeout = 50 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := c.EvaluateQuota(ctx, "organization", "org_a"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want the caller's deadline", err)
	}
	if waited := time.Since(start); waited > time.Second {
		t.Fatalf("caller waited %v for a stuck evaluation", waited)
	}
	if !<-src.sawDeadline {
		t.Fatal("the shared evaluation runs without a deadline")
	}

	// The evaluation itself gives up after its own timeout, so a later caller
	// starts a fresh one instead of joining the stuck one forever.
	time.Sleep(100 * time.Millisecond)
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()
	if _, err := c.EvaluateQuota(ctx2, "organization", "org_a"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second caller: err = %v", err)
	}
	if !<-src.sawDeadline {
		t.Fatal("no fresh evaluation for the second caller")
	}
}
