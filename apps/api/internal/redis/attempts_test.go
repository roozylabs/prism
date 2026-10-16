package redis

import (
	"context"
	"os"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

func newTestAttemptLimiter(t *testing.T, limit int, window time.Duration) *AttemptLimiter {
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
	return NewAttemptLimiter(rdb, "attempts-test-"+time.Now().Format("150405.000000")+":", limit, window)
}

func TestAttemptLimiter(t *testing.T) {
	l := newTestAttemptLimiter(t, 3, time.Minute)
	ctx := context.Background()
	t.Cleanup(func() { _ = l.Reset(ctx, "a"); _ = l.Reset(ctx, "b") })

	for i := 1; i <= 3; i++ {
		if ok, _, err := l.Hit(ctx, "a"); err != nil || !ok {
			t.Fatalf("attempt %d: ok=%v err=%v, want allowed", i, ok, err)
		}
	}
	if blocked, _, err := l.Blocked(ctx, "a"); err != nil || !blocked {
		t.Fatalf("after 3 attempts Blocked = %v, %v; want blocked", blocked, err)
	}
	ok, retry, err := l.Hit(ctx, "a")
	if err != nil || ok {
		t.Fatalf("4th attempt: ok=%v err=%v, want refused", ok, err)
	}
	if retry <= 0 || retry > time.Minute {
		t.Fatalf("retry after %v, want within the 1m window", retry)
	}
	if blocked, _, _ := l.Blocked(ctx, "b"); blocked {
		t.Fatal("another key is blocked too")
	}

	if err := l.Reset(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if ok, _, _ := l.Hit(ctx, "a"); !ok {
		t.Fatal("still refused after Reset")
	}
}

func TestAttemptLimiterWindowExpires(t *testing.T) {
	l := newTestAttemptLimiter(t, 1, 200*time.Millisecond)
	ctx := context.Background()
	t.Cleanup(func() { _ = l.Reset(ctx, "a") })

	_, _, _ = l.Hit(ctx, "a")
	if ok, _, _ := l.Hit(ctx, "a"); ok {
		t.Fatal("2nd attempt in the window allowed")
	}
	time.Sleep(300 * time.Millisecond)
	if ok, _, err := l.Hit(ctx, "a"); err != nil || !ok {
		t.Fatalf("after the window: ok=%v err=%v, want allowed", ok, err)
	}
}
