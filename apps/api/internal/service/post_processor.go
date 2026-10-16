package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	goredis "github.com/roozylabs/prism/internal/redis"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/roozylabs/prism/internal/utils"
)

type PostProcessTask struct {
	Ctx        context.Context
	RequestID  string
	Log        *models.RequestLog
	GatewayKey *models.GatewayAPIKey
	AgentID    string
	AgentName  string
	UserRole   string
	ClientIP   string
	UserAgent  string
	ClientApp  string
}

type AsyncPostProcessor struct {
	taskChan       chan *PostProcessTask
	wg             sync.WaitGroup
	requestLogs    *repository.RequestLogRepository
	gatewayKeys    *repository.GatewayKeyRepository
	eventPublisher *goredis.EventPublisher
	auditRecorder  *proxy.AuditRecorder
	workersCount   int

	// overflow bounds the extra goroutines used when the buffer is full.
	overflow chan struct{}
	// mu guards stopped and taskChan's closing against concurrent Enqueue.
	mu      sync.RWMutex
	stopped bool
}

func NewAsyncPostProcessor(
	requestLogs *repository.RequestLogRepository,
	gatewayKeys *repository.GatewayKeyRepository,
	eventPublisher *goredis.EventPublisher,
	auditRecorder *proxy.AuditRecorder,
	bufferSize int,
	workersCount int,
) *AsyncPostProcessor {
	if bufferSize <= 0 {
		bufferSize = 2000
	}
	if workersCount <= 0 {
		workersCount = 10
	}

	p := &AsyncPostProcessor{
		taskChan:       make(chan *PostProcessTask, bufferSize),
		requestLogs:    requestLogs,
		gatewayKeys:    gatewayKeys,
		eventPublisher: eventPublisher,
		auditRecorder:  auditRecorder,
		workersCount:   workersCount,
		overflow:       make(chan struct{}, workersCount*4),
	}

	p.start()
	return p
}

func (p *AsyncPostProcessor) start() {
	for i := 0; i < p.workersCount; i++ {
		p.wg.Add(1)
		go p.workerLoop()
	}
}

func (p *AsyncPostProcessor) workerLoop() {
	defer p.wg.Done()
	// Runs until Stop closes the channel, so queued tasks are drained first.
	for task := range p.taskChan {
		p.processTask(task)
	}
}

// Enqueue hands the task to the workers. When the buffer is full it runs the
// task on a bounded number of extra goroutines, and past that in the caller,
// so request logs slow requests down instead of being dropped or piling up
// goroutines. After Stop it runs the task in the caller. It reports whether
// the task was buffered.
func (p *AsyncPostProcessor) Enqueue(task *PostProcessTask) bool {
	if task == nil || task.Log == nil {
		return false
	}

	p.mu.RLock()
	if p.stopped {
		p.mu.RUnlock()
		p.processTask(task)
		return false
	}
	select {
	case p.taskChan <- task:
		p.mu.RUnlock()
		return true
	default:
	}
	select {
	case p.overflow <- struct{}{}:
		p.wg.Add(1)
		p.mu.RUnlock()
		go func() {
			defer p.wg.Done()
			defer func() { <-p.overflow }()
			p.processTask(task)
		}()
	default:
		p.mu.RUnlock()
		log.Printf("[AsyncPostProcessor] Warning: task buffer (size %d) and overflow full, processing in the request", cap(p.taskChan))
		p.processTask(task)
	}
	return false
}

func (p *AsyncPostProcessor) processTask(task *PostProcessTask) {
	if task == nil || task.Log == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[AsyncPostProcessor] Panic while processing request %s: %v", task.RequestID, r)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Save Request Log to PostgreSQL
	if p.requestLogs != nil {
		if err := p.requestLogs.Create(ctx, task.Log); err != nil {
			log.Printf("[AsyncPostProcessor] Error persisting request log (%s): %v", task.RequestID, err)
		}
	}

	// 2. Increment Gateway Key usage counter
	if p.gatewayKeys != nil && task.GatewayKey != nil && task.GatewayKey.ID != "" {
		if err := p.gatewayKeys.IncrementUsage(ctx, task.GatewayKey.ID); err != nil {
			log.Printf("[AsyncPostProcessor] Error incrementing key usage (%s): %v", task.GatewayKey.ID, err)
		}
	}

	// 3. Publish Redis SSE telemetry event
	if p.eventPublisher != nil {
		keyPrefix := ""
		gatewayKeyName := ""
		ownerID := ""
		if task.GatewayKey != nil {
			keyPrefix = task.GatewayKey.KeyPrefix
			gatewayKeyName = task.GatewayKey.Name
			ownerID = task.GatewayKey.UserID
		}

		eventData := map[string]interface{}{
			"requestId":      task.RequestID,
			"model":          task.Log.Model,
			"provider":       task.Log.ProviderType,
			"statusCode":     task.Log.StatusCode,
			"latencyMs":      task.Log.LatencyMs,
			"inputTokens":    task.Log.InputTokens,
			"outputTokens":   task.Log.OutputTokens,
			"totalTokens":    task.Log.TotalTokens,
			"costUsd":        task.Log.CostUSD,
			"clientApp":      task.Log.ClientApp,
			"orgId":          task.Log.OrgID,
			"workspaceId":    task.Log.WorkspaceID,
			"projectId":      task.Log.ProjectID,
			"keyPrefix":      keyPrefix,
			"gatewayKeyName": gatewayKeyName,
			"timestamp":      task.Log.CreatedAt,
		}
		_ = p.eventPublisher.Publish(ctx, ownerID, "request_log_created", eventData)
	}

	// 4. Record Cryptographic Audit Trail
	if p.auditRecorder != nil {
		userID := ""
		if task.GatewayKey != nil {
			userID = task.GatewayKey.UserID
		}

		promptHash := utils.HashSHA256(task.Log.Model + ":" + task.Log.RequestID)
		responseHash := utils.HashSHA256(task.Log.Model + ":" + task.Log.ProviderType + ":" + fmt.Sprintf("%d", task.Log.TotalTokens))

		trail := &models.AIAuditTrail{
			RequestID:         task.Log.RequestID,
			UserID:            userID,
			UserRole:          task.UserRole,
			ModelSlug:         task.Log.Model,
			FailoverChain:     []string{},
			ToolsInvoked:      []string{},
			ResourcesAccessed: []string{},
			MCPServersCalled:  []string{},
			PromptTokens:      task.Log.InputTokens,
			CompletionTokens:  task.Log.OutputTokens,
			TotalTokens:       task.Log.TotalTokens,
			TotalCostUSD:      task.Log.CostUSD,
			StatusCode:        task.Log.StatusCode,
			LatencyMS:         task.Log.LatencyMs,
			TTFTMS:            task.Log.TTFTMs,
			PromptHash:        promptHash,
			ResponseHash:      responseHash,
			ComplianceStatus:  "compliant",
		}

		if task.GatewayKey != nil && task.GatewayKey.ID != "" {
			trail.GatewayKeyID = &task.GatewayKey.ID
		}
		if task.AgentID != "" {
			agentID := task.AgentID
			agentName := task.AgentName
			trail.AgentID = &agentID
			trail.AgentName = &agentName
		}

		_ = p.auditRecorder.Record(ctx, trail)
	}
}

// Stop drains the queued tasks and waits for them. It is safe to call more
// than once and while requests are still enqueueing.
func (p *AsyncPostProcessor) Stop() {
	p.mu.Lock()
	if !p.stopped {
		p.stopped = true
		close(p.taskChan)
	}
	p.mu.Unlock()
	p.wg.Wait()
}
