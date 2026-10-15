package proxy

import (
	"context"
	"testing"
	"time"

	"github.com/roozylabs/prism/internal/utils"
)

func TestSleepCtx_ReturnsEarlyWhenCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	sleepCtx(ctx, 5*time.Second)
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Errorf("sleepCtx should return immediately on a canceled context, took %s", elapsed)
	}
}

func TestSleepCtx_WaitsFullDuration(t *testing.T) {
	start := time.Now()
	sleepCtx(context.Background(), 30*time.Millisecond)
	if elapsed := time.Since(start); elapsed < 30*time.Millisecond {
		t.Errorf("sleepCtx returned after %s, before the 30ms wait", elapsed)
	}
}

func TestErrClientCanceled_MapsTo499(t *testing.T) {
	status, _, code, _ := utils.CleanUpstreamError(ErrClientCanceled)
	if status != utils.StatusClientClosedRequest || code != "client_closed_request" {
		t.Errorf("expected 499 client_closed_request, got %d %s", status, code)
	}
}
