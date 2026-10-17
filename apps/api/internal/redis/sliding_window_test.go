package redis

import (
	"context"
	"os"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

func newTestSlidingWindow(t *testing.T, window time.Duration) (*SlidingWindowLimiter, *goredis.Client, string) {
	t.Helper()
	url := os.Getenv("REDIS_URL")
	if url == "" {
		t.Skip("REDIS_URL not set")
	}
	opts, err := goredis.ParseURL(url)
	if err != nil {
		t.Fatalf("parse REDIS_URL: %v", err)
	}
	rdb := goredis.NewClient(opts)
	t.Cleanup(func() { _ = rdb.Close() })
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	key := "sliding-test-" + time.Now().Format("150405.000000")
	t.Cleanup(func() { _ = rdb.Del(context.Background(), key).Err() })
	return NewSlidingWindowLimiter(rdb, window), rdb, key
}

func TestSlidingWindowLimiter(t *testing.T) {
	l, rdb, key := newTestSlidingWindow(t, time.Minute)
	ctx := context.Background()

	for i := int64(1); i <= 3; i++ {
		d, err := l.Allow(ctx, key, 3)
		if err != nil || !d.Allowed || d.Count != i {
			t.Fatalf("request %d: %+v err=%v, want allowed with count %d", i, d, err, i)
		}
	}
	for i := 4; i <= 5; i++ {
		d, err := l.Allow(ctx, key, 3)
		if err != nil || d.Allowed {
			t.Fatalf("request %d: %+v err=%v, want refused", i, d, err)
		}
		if d.Count != 3 {
			t.Fatalf("request %d: count %d, want 3", i, d.Count)
		}
		if d.RetryAfter <= 0 || d.RetryAfter > time.Minute || d.RetryAfter%time.Second != 0 {
			t.Fatalf("request %d: retry after %v, want whole seconds within the window", i, d.RetryAfter)
		}
	}
	// Refused requests must not take a slot in the window.
	if n, _ := rdb.ZCard(ctx, key).Result(); n != 3 {
		t.Fatalf("window holds %d members after refusals, want 3", n)
	}
	// The limit is the caller's: a higher one admits the same key again.
	if d, err := l.Allow(ctx, key, 5); err != nil || !d.Allowed || d.Count != 4 {
		t.Fatalf("with limit 5: %+v err=%v, want allowed with count 4", d, err)
	}
}

func TestSlidingWindowLimiterWindowExpires(t *testing.T) {
	l, _, key := newTestSlidingWindow(t, 200*time.Millisecond)
	ctx := context.Background()

	if d, err := l.Allow(ctx, key, 1); err != nil || !d.Allowed {
		t.Fatalf("first request: %+v err=%v, want allowed", d, err)
	}
	if d, _ := l.Allow(ctx, key, 1); d.Allowed {
		t.Fatal("second request in the window allowed")
	}
	time.Sleep(300 * time.Millisecond)
	if d, err := l.Allow(ctx, key, 1); err != nil || !d.Allowed || d.Count != 1 {
		t.Fatalf("after the window: %+v err=%v, want allowed with count 1", d, err)
	}
}
