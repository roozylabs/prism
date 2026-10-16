package queue

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/roozylabs/prism/internal/proxy"
)

func newTestQueue(t *testing.T) *JobQueue {
	t.Helper()
	url := os.Getenv("REDIS_URL")
	if url == "" {
		t.Skip("REDIS_URL is required")
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		t.Fatal(err)
	}
	rdb := redis.NewClient(opts)
	t.Cleanup(func() { _ = rdb.Close() })
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	prefix := fmt.Sprintf("test:%d:", time.Now().UnixNano())
	t.Cleanup(func() {
		keys, _ := rdb.Keys(context.Background(), prefix+"*").Result()
		if len(keys) > 0 {
			_ = rdb.Del(context.Background(), keys...).Err()
		}
	})
	q := newJobQueue(rdb, prefix)
	q.leaseTTL = 300 * time.Millisecond
	q.reapInterval = 100 * time.Millisecond
	return q
}

func waitForStatus(t *testing.T, q *JobQueue, jobID string, want JobStatus) *AsyncJob {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		job, err := q.GetJob(context.Background(), jobID)
		if err == nil && job.Status == want {
			return job
		}
		if time.Now().After(deadline) {
			t.Fatalf("job %s: status %v (err %v), want %s", jobID, job, err, want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func okExecutor(calls *atomic.Int32) func(*AsyncJob) (*proxy.ProviderResponse, error) {
	return func(job *AsyncJob) (*proxy.ProviderResponse, error) {
		calls.Add(1)
		if job.RequestPayload != nil && job.RequestPayload.Stream {
			return nil, fmt.Errorf("job %s was run as a stream", job.JobID)
		}
		return &proxy.ProviderResponse{ID: "resp-" + job.JobID}, nil
	}
}

func TestJobQueueRunsAJobAndClearsIt(t *testing.T) {
	q := newTestQueue(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	q.StartWorkerPool(ctx, 2, okExecutor(&calls))

	// Async jobs used to be run as streams when the client asked for one,
	// which panics without a client connection.
	job := &AsyncJob{JobID: "j1", RequestPayload: &proxy.ProxyRequest{Model: "m", Stream: true}}
	if err := q.EnqueueJob(ctx, job); err != nil {
		t.Fatal(err)
	}
	done := waitForStatus(t, q, "j1", StatusCompleted)
	if done.Result == nil || done.Result.ID != "resp-j1" || done.Attempts != 1 {
		t.Fatalf("completed job = %+v", done)
	}
	q.Stop(context.Background())
	if n, _ := q.rdb.LLen(context.Background(), q.processingKey).Result(); n != 0 {
		t.Fatalf("%d jobs left in the processing list", n)
	}
}

func TestJobOfADeadInstanceIsRunAgain(t *testing.T) {
	q := newTestQueue(t)
	bg := context.Background()
	// A job an instance took and then died with: in the processing list, no
	// lease. With BLPOP it was gone for good.
	if err := q.EnqueueJob(bg, &AsyncJob{JobID: "orphan", RequestPayload: &proxy.ProxyRequest{Model: "m"}}); err != nil {
		t.Fatal(err)
	}
	if err := q.rdb.LMove(bg, q.queueKey, q.processingKey, "LEFT", "RIGHT").Err(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(bg)
	defer cancel()
	var calls atomic.Int32
	q.StartWorkerPool(ctx, 1, okExecutor(&calls))
	waitForStatus(t, q, "orphan", StatusCompleted)
	q.Stop(bg)
	if calls.Load() != 1 {
		t.Fatalf("orphaned job ran %d times, want 1", calls.Load())
	}
}

func TestRunningJobIsNotReapedAndFinishesAfterShutdown(t *testing.T) {
	q := newTestQueue(t)
	bg := context.Background()
	ctx, cancel := context.WithCancel(bg)
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	q.StartWorkerPool(ctx, 1, func(job *AsyncJob) (*proxy.ProviderResponse, error) {
		calls.Add(1)
		close(started)
		<-release
		return &proxy.ProviderResponse{ID: "late"}, nil
	})
	if err := q.EnqueueJob(bg, &AsyncJob{JobID: "slow"}); err != nil {
		t.Fatal(err)
	}
	<-started

	// Several lease lifetimes and reap passes go by while the job runs.
	time.Sleep(time.Second)
	if n, _ := q.rdb.LLen(bg, q.queueKey).Result(); n != 0 {
		t.Fatal("a running job was put back on the queue")
	}

	// SIGTERM: the worker context is cancelled while the job is running. Its
	// status used to be written with that context and never landed.
	cancel()
	close(release)
	q.Stop(bg)
	if job := waitForStatus(t, q, "slow", StatusCompleted); job.Result.ID != "late" {
		t.Fatalf("job = %+v", job)
	}
	if calls.Load() != 1 {
		t.Fatalf("job ran %d times", calls.Load())
	}
}

func TestJobThatKeepsDyingIsGivenUp(t *testing.T) {
	q := newTestQueue(t)
	bg := context.Background()
	job := &AsyncJob{JobID: "doomed"}
	if err := q.EnqueueJob(bg, job); err != nil {
		t.Fatal(err)
	}
	job.Attempts = maxJobAttempts
	if err := q.UpdateJob(bg, job); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(bg)
	defer cancel()
	var calls atomic.Int32
	q.StartWorkerPool(ctx, 1, okExecutor(&calls))
	waitForStatus(t, q, "doomed", StatusFailed)
	q.Stop(bg)
	if calls.Load() != 0 {
		t.Fatal("a job past its attempts was run")
	}
}
