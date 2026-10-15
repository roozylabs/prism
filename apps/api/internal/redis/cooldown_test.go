package redis

import (
	"context"
	"os"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// newTestCooldownStore connects to REDIS_URL (set in CI) and skips otherwise.
func newTestCooldownStore(t *testing.T) *CooldownStore {
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
	return NewCooldownStore(rdb)
}

func TestCoolingSet(t *testing.T) {
	s := newTestCooldownStore(t)
	ctx := context.Background()
	prefix := "cooling-set-test-" + time.Now().Format("150405.000000") + "-"
	a, b, c := prefix+"a", prefix+"b", prefix+"c"
	t.Cleanup(func() {
		for _, id := range []string{a, b, c} {
			_ = s.RemoveCooldown(ctx, id)
		}
	})

	if err := s.SetCooldown(ctx, a, 60); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCooldown(ctx, c, 60); err != nil {
		t.Fatal(err)
	}

	got, err := s.CoolingSet(ctx, []string{a, b, c})
	if err != nil {
		t.Fatal(err)
	}
	if !got[a] || got[b] || !got[c] || len(got) != 2 {
		t.Errorf("expected only %s and %s cooling, got %v", a, c, got)
	}

	if err := s.ClearCooldown(ctx, a); err != nil {
		t.Fatal(err)
	}
	got, _ = s.CoolingSet(ctx, []string{a, b, c})
	if got[a] {
		t.Errorf("cleared credential still reported cooling: %v", got)
	}
}

func TestCoolingSet_NilAndEmpty(t *testing.T) {
	var s *CooldownStore
	got, err := s.CoolingSet(context.Background(), []string{"x"})
	if err != nil || len(got) != 0 {
		t.Errorf("nil store should return an empty set, got %v, %v", got, err)
	}
	got, err = NewCooldownStore(nil).CoolingSet(context.Background(), nil)
	if err != nil || len(got) != 0 {
		t.Errorf("no IDs should return an empty set, got %v, %v", got, err)
	}
}

func TestGetActiveStreamsScopedToUser(t *testing.T) {
	s := newTestCooldownStore(t)
	ctx := context.Background()
	suffix := time.Now().Format("150405.000000")
	alice, bob := "alice-"+suffix, "bob-"+suffix
	aliceReq, bobReq := "req-a-"+suffix, "req-b-"+suffix
	t.Cleanup(func() {
		_ = s.UntrackActiveStream(ctx, aliceReq)
		_ = s.UntrackActiveStream(ctx, bobReq)
	})

	if err := s.TrackActiveStream(ctx, aliceReq, alice, "model-a-"+suffix, "key-a", "cred-a"); err != nil {
		t.Fatal(err)
	}
	if err := s.TrackActiveStream(ctx, bobReq, bob, "model-b-"+suffix, "key-b", "cred-b"); err != nil {
		t.Fatal(err)
	}

	mine, err := s.GetActiveStreams(ctx, alice)
	if err != nil {
		t.Fatal(err)
	}
	if mine.TotalActive != 1 || mine.ByModel["model-a-"+suffix] != 1 || mine.ByModel["model-b-"+suffix] != 0 || mine.ByKey["key-b"] != 0 {
		t.Fatalf("alice's summary includes other users' streams: %+v", mine)
	}

	all, err := s.GetActiveStreams(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if all.ByModel["model-a-"+suffix] != 1 || all.ByModel["model-b-"+suffix] != 1 {
		t.Fatalf("unscoped summary is missing streams: %+v", all)
	}
}
