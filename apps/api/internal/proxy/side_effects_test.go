package proxy

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestSideEffectsRunTasksWithADeadline(t *testing.T) {
	s := NewSideEffects(2, 8, 50*time.Millisecond)
	defer func() { _ = s.Stop(context.Background()) }()

	done := make(chan time.Duration, 1)
	if !s.Go("deadline", func(ctx context.Context) {
		deadline, ok := ctx.Deadline()
		if !ok {
			done <- 0
			return
		}
		done <- time.Until(deadline)
	}) {
		t.Fatal("task refused with an empty queue")
	}
	select {
	case left := <-done:
		if left <= 0 || left > 50*time.Millisecond {
			t.Fatalf("task context deadline %v away, want within the 50ms timeout", left)
		}
	case <-time.After(time.Second):
		t.Fatal("task never ran")
	}
}

func TestSideEffectsDropWhenFullInsteadOfGrowing(t *testing.T) {
	s := NewSideEffects(1, 2, time.Second)
	defer func() { _ = s.Stop(context.Background()) }()

	release := make(chan struct{})
	started := make(chan struct{})
	// Occupy the one worker, then fill the queue.
	s.Go("blocker", func(ctx context.Context) {
		close(started)
		<-release
	})
	<-started
	for i := 0; i < 2; i++ {
		if !s.Go("queued", func(context.Context) {}) {
			t.Fatalf("task %d refused with room in the queue", i)
		}
	}
	if s.Go("overflow", func(context.Context) {}) {
		t.Fatal("task accepted although the worker and the queue were full")
	}
	if s.Dropped() != 1 || s.Pending() != 2 {
		t.Fatalf("dropped %d pending %d, want 1 and 2", s.Dropped(), s.Pending())
	}
	close(release)
}

func TestSideEffectsSlowTaskIsCutOffByTimeout(t *testing.T) {
	s := NewSideEffects(1, 1, 30*time.Millisecond)
	defer func() { _ = s.Stop(context.Background()) }()

	finished := make(chan struct{})
	s.Go("slow", func(ctx context.Context) {
		<-ctx.Done()
		close(finished)
	})
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("task kept running past its timeout")
	}
}

func TestSideEffectsSurviveAPanic(t *testing.T) {
	s := NewSideEffects(1, 4, time.Second)
	defer func() { _ = s.Stop(context.Background()) }()

	s.Go("panics", func(context.Context) { panic("boom") })
	ran := make(chan struct{})
	s.Go("after", func(context.Context) { close(ran) })
	select {
	case <-ran:
	case <-time.After(time.Second):
		t.Fatal("worker died with the panicking task")
	}
}

func TestSideEffectsStopDrainsAndThenRunsInline(t *testing.T) {
	s := NewSideEffects(1, 8, time.Second)
	var ran atomic.Int32
	for i := 0; i < 5; i++ {
		s.Go("queued", func(context.Context) { ran.Add(1) })
	}
	if err := s.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if ran.Load() != 5 {
		t.Fatalf("%d tasks ran before Stop returned, want all 5", ran.Load())
	}
	// After Stop the task still runs, in the caller.
	if s.Go("late", func(context.Context) { ran.Add(1) }) {
		t.Fatal("task reported as queued after Stop")
	}
	if ran.Load() != 6 {
		t.Fatal("task after Stop did not run")
	}
}

func TestSideEffectsNilPoolStillRuns(t *testing.T) {
	var s *SideEffects
	ran := make(chan struct{})
	s.Go("nil pool", func(context.Context) { close(ran) })
	select {
	case <-ran:
	case <-time.After(time.Second):
		t.Fatal("nil pool dropped the task")
	}
}
