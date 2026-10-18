package proxy

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultSideEffectWorkers = 16
	defaultSideEffectQueue   = 1024
	// defaultSideEffectTimeout bounds one background write. A database that
	// has stopped answering releases the worker instead of holding it.
	defaultSideEffectTimeout = 10 * time.Second
)

type sideEffect struct {
	name string
	fn   func(ctx context.Context)
}

// SideEffects runs the writes a request leaves behind (payloads, routing
// decisions, credential health, tool records, telemetry) on a fixed number
// of workers, each call with a deadline. The request path only enqueues.
// When the queue is full the task is dropped and counted: a slow database
// then costs some bookkeeping, not an unbounded pile of goroutines.
type SideEffects struct {
	queue   chan sideEffect
	timeout time.Duration
	wg      sync.WaitGroup
	dropped atomic.Int64

	mu      sync.RWMutex
	stopped bool

	logMu   sync.Mutex
	logLast time.Time
}

func NewSideEffects(workers, queueSize int, timeout time.Duration) *SideEffects {
	if workers <= 0 {
		workers = defaultSideEffectWorkers
	}
	if queueSize <= 0 {
		queueSize = defaultSideEffectQueue
	}
	if timeout <= 0 {
		timeout = defaultSideEffectTimeout
	}
	s := &SideEffects{queue: make(chan sideEffect, queueSize), timeout: timeout}
	for i := 0; i < workers; i++ {
		s.wg.Add(1)
		go s.worker()
	}
	return s
}

func (s *SideEffects) worker() {
	defer s.wg.Done()
	for t := range s.queue {
		runSideEffect(t, s.timeout)
	}
}

func runSideEffect(t sideEffect, timeout time.Duration) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("side effect %s: panic: %v", t.name, r)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	t.fn(ctx)
}

// Go queues fn and reports whether it was accepted. A nil pool runs fn on
// its own goroutine with the default deadline, so an Engine built without
// one keeps working. After Stop the task runs in the caller.
func (s *SideEffects) Go(name string, fn func(ctx context.Context)) bool {
	if fn == nil {
		return false
	}
	if s == nil {
		go runSideEffect(sideEffect{name: name, fn: fn}, defaultSideEffectTimeout)
		return true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.stopped {
		runSideEffect(sideEffect{name: name, fn: fn}, s.timeout)
		return false
	}
	select {
	case s.queue <- sideEffect{name: name, fn: fn}:
		return true
	default:
		s.dropped.Add(1)
		s.logDrop(name)
		return false
	}
}

// Dropped counts tasks refused because the queue was full.
func (s *SideEffects) Dropped() int64 {
	if s == nil {
		return 0
	}
	return s.dropped.Load()
}

// Pending counts queued tasks not yet picked up by a worker.
func (s *SideEffects) Pending() int {
	if s == nil {
		return 0
	}
	return len(s.queue)
}

// Stop lets the workers drain the queue and waits for them, or until ctx
// ends. It is safe to call more than once.
func (s *SideEffects) Stop(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	if !s.stopped {
		s.stopped = true
		close(s.queue)
	}
	s.mu.Unlock()

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// logDrop reports a full queue at most once every 10 seconds: under a slow
// database every request would otherwise add a line.
func (s *SideEffects) logDrop(name string) {
	s.logMu.Lock()
	defer s.logMu.Unlock()
	if time.Since(s.logLast) < 10*time.Second {
		return
	}
	s.logLast = time.Now()
	log.Printf("side effects: queue full, dropping %s (%d dropped so far)", name, s.dropped.Load())
}
