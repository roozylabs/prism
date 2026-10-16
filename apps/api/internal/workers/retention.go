package workers

import (
	"context"
	"log"
	"time"

	"github.com/roozylabs/prism/internal/config"
	"github.com/roozylabs/prism/internal/redis"
	"github.com/roozylabs/prism/internal/repository"
)

type LatencyFlushWorker struct {
	telemetry *redis.ModelTelemetryStore
	models    *repository.ModelRepository
	latency   *repository.ModelLatencyHourlyRepository
}

func NewLatencyFlushWorker(telemetry *redis.ModelTelemetryStore, models *repository.ModelRepository, latency *repository.ModelLatencyHourlyRepository) *LatencyFlushWorker {
	return &LatencyFlushWorker{telemetry: telemetry, models: models, latency: latency}
}

func (w *LatencyFlushWorker) Run(ctx context.Context) {
	modelList, err := w.models.ListEnabled(ctx)
	if err != nil {
		log.Printf("[latency-flush] list models: %v", err)
		return
	}
	hourStart := time.Now().Truncate(time.Hour)
	for _, m := range modelList {
		if !m.Enabled {
			continue
		}
		metrics, err := w.telemetry.GetModelMetrics(ctx, m.Slug)
		if err != nil || metrics.SampleCount == 0 {
			continue
		}
		avgTTFT := metrics.AvgTTFTMs
		avgLat := metrics.AvgLatencyMs
		p95 := metrics.P95TTFTMs
		samples := int(metrics.SampleCount)
		successes := int(metrics.SuccessCount)
		entry := &repository.ModelLatencyHourly{
			ModelSlug:    m.Slug,
			HourStart:    hourStart,
			AvgTTFTMs:    &avgTTFT,
			AvgLatencyMs: &avgLat,
			P95LatencyMs: &p95,
			SampleCount:  &samples,
			SuccessCount: &successes,
		}
		if err := w.latency.Upsert(ctx, entry); err != nil {
			log.Printf("[latency-flush] upsert %s: %v", m.Slug, err)
		}
	}
}

// RetentionPolicy deletes rows of Table whose Column is older than Keep.
// Keep 0 deletes rows whose Column (an expiry time) has passed.
type RetentionPolicy struct {
	Table  string
	Column string
	Keep   time.Duration
}

type rowPurger interface {
	PurgeBefore(ctx context.Context, table, column string, cutoff time.Time, batchSize, maxBatches int, pause time.Duration) (int64, error)
}

// RetentionWorker applies retention policies in batches of 5,000 rows, at
// most a million rows per table per run; a backlog is worked off over the
// following runs.
type RetentionWorker struct {
	purger   rowPurger
	policies []RetentionPolicy
}

func NewRetentionWorker(purger rowPurger, policies []RetentionPolicy) *RetentionWorker {
	return &RetentionWorker{purger: purger, policies: policies}
}

// RetentionPolicies turns the RETENTION_*_DAYS settings into policies,
// skipping tables kept forever, and adds the expired sign-in sessions,
// verification tokens and OAuth states.
func RetentionPolicies(days config.RetentionDays) []RetentionPolicy {
	var out []RetentionPolicy
	for _, t := range []struct {
		table string
		days  int
	}{
		{"request_logs", days.RequestLogs},
		{"routing_decisions", days.RoutingDecisions},
		{"request_payloads", days.RequestPayloads},
		{"tool_invocations", days.ToolInvocations},
		{"mcp_server_invocations", days.MCPInvocations},
		{"ai_audit_trails", days.AIAuditTrails},
		{"audit_logs", days.AuditLogs},
		{"cost_anomalies", days.CostAnomalies},
		{"budget_alerts", days.BudgetAlerts},
	} {
		if t.days > 0 {
			out = append(out, RetentionPolicy{Table: t.table, Column: "created_at", Keep: time.Duration(t.days) * 24 * time.Hour})
		}
	}
	return append(out,
		RetentionPolicy{Table: "session", Column: "expires_at"},
		RetentionPolicy{Table: "verification", Column: "expires_at"},
		RetentionPolicy{Table: "oauth_states", Column: "expires_at"},
	)
}

func (w *RetentionWorker) Run(ctx context.Context) {
	now := time.Now()
	for _, p := range w.policies {
		n, err := w.purger.PurgeBefore(ctx, p.Table, p.Column, now.Add(-p.Keep), 5000, 200, 100*time.Millisecond)
		if err != nil {
			log.Printf("[retention] %s: %v (%d rows deleted before the error)", p.Table, err, n)
			if ctx.Err() != nil {
				return
			}
			continue
		}
		if n > 0 {
			log.Printf("[retention] %s: deleted %d rows older than %s", p.Table, n, now.Add(-p.Keep).Format(time.RFC3339))
		}
	}
}
