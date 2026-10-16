package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/roozylabs/prism/internal/models"
	goredis "github.com/roozylabs/prism/internal/redis"
	"github.com/stretchr/testify/assert"
)

func TestAsyncPostProcessor_EnqueueAndProcess(t *testing.T) {
	processor := NewAsyncPostProcessor(nil, nil, nil, nil, 100, 2)
	defer processor.Stop()

	task := &PostProcessTask{
		Ctx:       context.Background(),
		RequestID: "req_test_123",
		Log: &models.RequestLog{
			ID:        "log_123",
			Model:     "gemini-3.6-flash",
			CostUSD:   0.001,
			CreatedAt: time.Now(),
		},
		GatewayKey: &models.GatewayAPIKey{
			ID: "key_123",
		},
	}

	enqueued := processor.Enqueue(task)
	assert.True(t, enqueued)
}

func TestAsyncPostProcessor_NilTaskHandling(t *testing.T) {
	processor := NewAsyncPostProcessor(nil, nil, nil, nil, 10, 1)
	defer processor.Stop()

	assert.False(t, processor.Enqueue(nil))
	assert.False(t, processor.Enqueue(&PostProcessTask{}))
}

func TestPostProcessorStopWhileEnqueueing(t *testing.T) {
	p := NewAsyncPostProcessor(nil, nil, nil, nil, 1, 1)

	// Enqueue used to send on the channel Stop had closed and panic.
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				p.Enqueue(&PostProcessTask{Log: &models.RequestLog{}})
			}
		}()
	}
	p.Stop()
	wg.Wait()
	p.Stop()
}

func TestPostProcessorSurvivesAPanickingTask(t *testing.T) {
	// A publisher without a Redis client panics on Publish.
	p := NewAsyncPostProcessor(nil, nil, &goredis.EventPublisher{}, nil, 4, 1)
	key := &models.GatewayAPIKey{ID: "k", UserID: "u1"}
	for i := 0; i < 3; i++ {
		if !p.Enqueue(&PostProcessTask{Log: &models.RequestLog{}, GatewayKey: key}) {
			t.Fatal("task not buffered")
		}
	}
	// A panic in a worker used to take the whole server down; here it would
	// end the test binary before Stop returns.
	p.Stop()
}
