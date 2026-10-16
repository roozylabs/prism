package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	goredis "github.com/roozylabs/prism/internal/redis"
)

type JobStatus string

const (
	StatusQueued     JobStatus = "queued"
	StatusProcessing JobStatus = "processing"
	StatusCompleted  JobStatus = "completed"
	StatusFailed     JobStatus = "failed"
)

type AsyncJob struct {
	JobID          string                  `json:"job_id"`
	RequestID      string                  `json:"request_id"`
	UserID         string                  `json:"user_id"`
	Status         JobStatus               `json:"status"`
	Model          string                  `json:"model"`
	AgentID        string                  `json:"agent_id,omitempty"`
	AgentName      string                  `json:"agent_name,omitempty"`
	UserRole       string                  `json:"user_role,omitempty"`
	ClientIP       string                  `json:"client_ip,omitempty"`
	UserAgent      string                  `json:"user_agent,omitempty"`
	ClientApp      string                  `json:"client_app,omitempty"`
	RequestPayload *proxy.ProxyRequest     `json:"request_payload,omitempty"`
	GatewayKey     *models.GatewayAPIKey   `json:"gateway_key,omitempty"`
	TenantCtx      models.TenantContext    `json:"tenant_ctx"`
	Result         *proxy.ProviderResponse `json:"result,omitempty"`
	Error          string                  `json:"error,omitempty"`
	Attempts       int                     `json:"attempts,omitempty"`
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

// maxJobAttempts is how many times a job may be picked up. A job is picked
// up again only when the instance running it died, so a job that keeps
// killing its worker stops being retried.
const maxJobAttempts = 3

// requeueScript puts a job from the processing list back on the queue unless
// a worker holds its lease.
var requeueScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[3]) == 1 then return 0 end
if redis.call('LREM', KEYS[1], 1, ARGV[1]) > 0 then
	redis.call('RPUSH', KEYS[2], ARGV[1])
	return 1
end
return 0`)

// JobQueue runs async jobs from a Redis list. A worker moves a job to the
// processing list atomically and holds a lease on it while it runs, so a job
// whose instance is killed is found by any instance's reaper and run again.
type JobQueue struct {
	rdb            *redis.Client
	eventPublisher *goredis.EventPublisher
	queueKey       string
	processingKey  string
	jobKeyPfx      string
	leaseKeyPfx    string
	ttl            time.Duration
	leaseTTL       time.Duration
	reapInterval   time.Duration
	wg             sync.WaitGroup
	stopChan       chan struct{}
	stopOnce       sync.Once
}

func NewJobQueue(rdb *redis.Client) *JobQueue {
	return newJobQueue(rdb, "prism:")
}

func newJobQueue(rdb *redis.Client, prefix string) *JobQueue {
	return &JobQueue{
		rdb:           rdb,
		queueKey:      prefix + "queue:async_jobs",
		processingKey: prefix + "queue:async_jobs:processing",
		jobKeyPfx:     prefix + "job:",
		leaseKeyPfx:   prefix + "job_lease:",
		ttl:           24 * time.Hour,
		leaseTTL:      time.Minute,
		reapInterval:  30 * time.Second,
		stopChan:      make(chan struct{}),
	}
}

func (q *JobQueue) SetEventPublisher(pub *goredis.EventPublisher) {
	q.eventPublisher = pub
}

func (q *JobQueue) publishJobEvent(ctx context.Context, job *AsyncJob) {
	if q.eventPublisher == nil || job == nil {
		return
	}

	eventData := map[string]interface{}{
		"jobId":     job.JobID,
		"requestId": job.RequestID,
		"userId":    job.UserID,
		"status":    string(job.Status),
		"model":     job.Model,
		"result":    job.Result,
		"error":     job.Error,
		"timestamp": job.UpdatedAt,
	}

	_ = q.eventPublisher.Publish(ctx, job.UserID, "async_job_updated", eventData)
}

func (q *JobQueue) EnqueueJob(ctx context.Context, job *AsyncJob) error {
	if job == nil || job.JobID == "" {
		return fmt.Errorf("invalid job payload")
	}

	job.Status = StatusQueued
	job.CreatedAt = time.Now()
	job.UpdatedAt = time.Now()

	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}

	jobKey := q.jobKeyPfx + job.JobID

	// Save job state to Redis with 24h TTL
	if err := q.rdb.Set(ctx, jobKey, data, q.ttl).Err(); err != nil {
		return fmt.Errorf("set redis job state: %w", err)
	}

	// Push job ID to queue list
	if err := q.rdb.RPush(ctx, q.queueKey, job.JobID).Err(); err != nil {
		return fmt.Errorf("rpush redis queue: %w", err)
	}

	q.publishJobEvent(ctx, job)
	return nil
}

func (q *JobQueue) GetJob(ctx context.Context, jobID string) (*AsyncJob, error) {
	if jobID == "" {
		return nil, fmt.Errorf("job id is required")
	}

	jobKey := q.jobKeyPfx + jobID
	data, err := q.rdb.Get(ctx, jobKey).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, fmt.Errorf("job not found")
		}
		return nil, fmt.Errorf("get redis job state: %w", err)
	}

	var job AsyncJob
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("unmarshal job: %w", err)
	}

	return &job, nil
}

func (q *JobQueue) UpdateJob(ctx context.Context, job *AsyncJob) error {
	if job == nil || job.JobID == "" {
		return fmt.Errorf("invalid job payload")
	}

	job.UpdatedAt = time.Now()
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}

	jobKey := q.jobKeyPfx + job.JobID
	err = q.rdb.Set(ctx, jobKey, data, q.ttl).Err()
	if err == nil {
		q.publishJobEvent(ctx, job)
	}
	return err
}

// StartWorkerPool starts the workers and the reaper. Workers stop taking
// jobs when ctx is done or Stop is called; a job already running finishes.
func (q *JobQueue) StartWorkerPool(ctx context.Context, workersCount int, executor func(job *AsyncJob) (*proxy.ProviderResponse, error)) {
	if workersCount <= 0 {
		workersCount = 5
	}

	for i := 0; i < workersCount; i++ {
		q.wg.Add(1)
		go q.workerLoop(ctx, i, executor)
	}
	q.wg.Add(1)
	go q.reapLoop(ctx)
}

func (q *JobQueue) stopping(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	case <-q.stopChan:
		return true
	default:
		return false
	}
}

func (q *JobQueue) workerLoop(ctx context.Context, workerID int, executor func(job *AsyncJob) (*proxy.ProviderResponse, error)) {
	defer q.wg.Done()

	for !q.stopping(ctx) {
		jobID, err := q.rdb.BLMove(ctx, q.queueKey, q.processingKey, "LEFT", "RIGHT", 2*time.Second).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) || ctx.Err() != nil {
				continue
			}
			log.Printf("[JobQueue Worker %d] BLMove error: %v", workerID, err)
			time.Sleep(time.Second)
			continue
		}
		q.runJob(workerID, jobID, executor)
	}
}

// runJob runs one job taken from the queue. It deliberately does not use the
// worker's context: shutdown stops new jobs, but the running one still gets
// its result and status written.
func (q *JobQueue) runJob(workerID int, jobID string, executor func(job *AsyncJob) (*proxy.ProviderResponse, error)) {
	ctx := context.Background()
	leaseKey := q.leaseKeyPfx + jobID
	if err := q.rdb.Set(ctx, leaseKey, workerID, q.leaseTTL).Err(); err != nil {
		log.Printf("[JobQueue Worker %d] Failed to lease job %s: %v", workerID, jobID, err)
	}
	stopHeartbeat := q.heartbeat(leaseKey)
	defer func() {
		stopHeartbeat()
		_ = q.rdb.LRem(ctx, q.processingKey, 1, jobID).Err()
		_ = q.rdb.Del(ctx, leaseKey).Err()
	}()

	job, err := q.GetJob(ctx, jobID)
	if err != nil || job == nil {
		log.Printf("[JobQueue Worker %d] Failed to retrieve job %s: %v", workerID, jobID, err)
		return
	}
	// The instance that ran it may have died after finishing it.
	if job.Status == StatusCompleted || job.Status == StatusFailed {
		return
	}

	job.Attempts++
	if job.Attempts > maxJobAttempts {
		job.Status = StatusFailed
		job.Error = fmt.Sprintf("job was interrupted %d times and is not retried again", maxJobAttempts)
		q.saveJob(job)
		log.Printf("[JobQueue Worker %d] Job %s abandoned after %d attempts", workerID, jobID, maxJobAttempts)
		return
	}
	// An async job returns the whole response when it is done; there is no
	// client connection to stream to.
	if job.RequestPayload != nil {
		job.RequestPayload.Stream = false
	}

	job.Status = StatusProcessing
	q.saveJob(job)

	defer func() {
		if r := recover(); r != nil {
			job.Status = StatusFailed
			job.Error = fmt.Sprintf("panic in worker: %v", r)
			q.saveJob(job)
			log.Printf("[JobQueue Worker %d] Job %s panicked: %v", workerID, jobID, r)
		}
	}()

	resp, execErr := executor(job)
	if execErr != nil {
		job.Status = StatusFailed
		job.Error = execErr.Error()
		log.Printf("[JobQueue Worker %d] Job %s failed: %v", workerID, jobID, execErr)
	} else {
		job.Status = StatusCompleted
		job.Result = resp
		log.Printf("[JobQueue Worker %d] Job %s completed successfully", workerID, jobID)
	}
	q.saveJob(job)
}

func (q *JobQueue) saveJob(job *AsyncJob) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := q.UpdateJob(ctx, job); err != nil {
		log.Printf("[JobQueue] Failed to save job %s (%s): %v", job.JobID, job.Status, err)
	}
}

// heartbeat keeps the lease alive until the returned function is called.
func (q *JobQueue) heartbeat(leaseKey string) func() {
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(q.leaseTTL / 3)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				_ = q.rdb.Expire(context.Background(), leaseKey, q.leaseTTL).Err()
			}
		}
	}()
	return func() { close(done) }
}

func (q *JobQueue) reapLoop(ctx context.Context) {
	defer q.wg.Done()
	t := time.NewTicker(q.reapInterval)
	defer t.Stop()

	var suspects map[string]bool
	for {
		suspects = q.reap(ctx, suspects)
		select {
		case <-ctx.Done():
			return
		case <-q.stopChan:
			return
		case <-t.C:
		}
	}
}

// reap requeues jobs in the processing list that have had no lease for a
// whole reap interval. Waiting one interval covers the moment between a
// worker taking a job and leasing it. It returns the jobs without a lease
// this time.
func (q *JobQueue) reap(ctx context.Context, suspects map[string]bool) map[string]bool {
	ids, err := q.rdb.LRange(ctx, q.processingKey, 0, -1).Result()
	if err != nil {
		if ctx.Err() == nil {
			log.Printf("[JobQueue] Reaper failed to list processing jobs: %v", err)
		}
		return suspects
	}
	leaseless := make(map[string]bool)
	for _, id := range ids {
		leased, err := q.rdb.Exists(ctx, q.leaseKeyPfx+id).Result()
		if err != nil || leased > 0 {
			continue
		}
		if !suspects[id] {
			leaseless[id] = true
			continue
		}
		n, err := requeueScript.Run(ctx, q.rdb, []string{q.processingKey, q.queueKey, q.leaseKeyPfx + id}, id).Int()
		if err != nil {
			log.Printf("[JobQueue] Reaper failed to requeue job %s: %v", id, err)
			leaseless[id] = true
		} else if n == 1 {
			log.Printf("[JobQueue] Requeued job %s whose worker stopped", id)
		}
	}
	return leaseless
}

// Stop stops taking jobs and waits until the running ones have finished or
// ctx is done. A job still running after that keeps its lease until the
// process exits; another instance's reaper then runs it again.
func (q *JobQueue) Stop(ctx context.Context) {
	q.stopOnce.Do(func() { close(q.stopChan) })
	done := make(chan struct{})
	go func() {
		q.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		log.Printf("[JobQueue] Stopped waiting for running jobs: %v", ctx.Err())
	}
}
