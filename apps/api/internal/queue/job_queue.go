package queue

import (
	"context"
	"encoding/json"
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
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

type JobQueue struct {
	rdb            *redis.Client
	eventPublisher *goredis.EventPublisher
	queueKey       string
	jobKeyPfx      string
	ttl            time.Duration
	wg             sync.WaitGroup
	stopChan       chan struct{}
}

func NewJobQueue(rdb *redis.Client) *JobQueue {
	return &JobQueue{
		rdb:       rdb,
		queueKey:  "prism:queue:async_jobs",
		jobKeyPfx: "prism:job:",
		ttl:       24 * time.Hour,
		stopChan:  make(chan struct{}),
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
		if err == redis.Nil {
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

func (q *JobQueue) StartWorkerPool(ctx context.Context, workersCount int, executor func(job *AsyncJob) (*proxy.ProviderResponse, error)) {
	if workersCount <= 0 {
		workersCount = 5
	}

	for i := 0; i < workersCount; i++ {
		q.wg.Add(1)
		go q.workerLoop(ctx, i, executor)
	}
}

func (q *JobQueue) workerLoop(ctx context.Context, workerID int, executor func(job *AsyncJob) (*proxy.ProviderResponse, error)) {
	defer q.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-q.stopChan:
			return
		default:
			// Pop next job from Redis list with 2 second timeout
			res, err := q.rdb.BLPop(ctx, 2*time.Second, q.queueKey).Result()
			if err != nil {
				if err == redis.Nil || err == context.Canceled {
					continue
				}
				log.Printf("[JobQueue Worker %d] BLPop error: %v", workerID, err)
				time.Sleep(1 * time.Second)
				continue
			}

			if len(res) < 2 {
				continue
			}

			jobID := res[1]
			job, err := q.GetJob(ctx, jobID)
			if err != nil || job == nil {
				log.Printf("[JobQueue Worker %d] Failed to retrieve job %s: %v", workerID, jobID, err)
				continue
			}

			// Update status to processing
			job.Status = StatusProcessing
			_ = q.UpdateJob(ctx, job)

			// Safely execute completion payload with panic recovery
			func() {
				defer func() {
					if r := recover(); r != nil {
						job.Status = StatusFailed
						job.Error = fmt.Sprintf("panic in worker: %v", r)
						_ = q.UpdateJob(ctx, job)
						log.Printf("[JobQueue Worker %d] Job %s panicked: %v", workerID, jobID, r)
					}
				}()

				resp, execErr := executor(job)
				if execErr != nil {
					job.Status = StatusFailed
					job.Error = execErr.Error()
					_ = q.UpdateJob(ctx, job)
					log.Printf("[JobQueue Worker %d] Job %s failed: %v", workerID, jobID, execErr)
				} else {
					job.Status = StatusCompleted
					job.Result = resp
					_ = q.UpdateJob(ctx, job)
					log.Printf("[JobQueue Worker %d] Job %s completed successfully", workerID, jobID)
				}
			}()
		}
	}
}

func (q *JobQueue) Stop() {
	close(q.stopChan)
	q.wg.Wait()
}
