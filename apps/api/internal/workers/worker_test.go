package workers

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestJobRunsSoonAfterStart(t *testing.T) {
	old := maxStartJitter
	maxStartJitter = 20 * time.Millisecond
	defer func() { maxStartJitter = old }()

	var runs atomic.Int32
	m := NewManager()
	m.Register("hourly", time.Hour, func(context.Context) { runs.Add(1) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)

	// An hourly job used to wait a full hour after every start, so with
	// deploys more often than that it never ran.
	deadline := time.Now().Add(2 * time.Second)
	for runs.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("hourly job did not run after start")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestFirstRunDelayIsBounded(t *testing.T) {
	for _, interval := range []time.Duration{time.Minute, time.Hour, 0} {
		for i := 0; i < 100; i++ {
			d := firstRunDelay(interval)
			if d < 0 || d > interval/4 || d > maxStartJitter {
				t.Fatalf("firstRunDelay(%v) = %v", interval, d)
			}
		}
	}
}
