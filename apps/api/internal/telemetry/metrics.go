package telemetry

import (
	"context"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const MeterName = "prism-gateway-meter"

var (
	meter metric.Meter

	// Gateway
	requestCounter      metric.Int64Counter
	requestErrorCounter metric.Int64Counter
	requestDurationHist metric.Float64Histogram

	// Admission
	admissionAllowedCounter metric.Int64Counter
	admissionDeniedCounter  metric.Int64Counter
	budgetExceededCounter   metric.Int64Counter
	quotaExceededCounter    metric.Int64Counter

	// Routing
	routingDecisionsCounter metric.Int64Counter
	routingFallbacksCounter metric.Int64Counter

	// Credentials
	credentialHealthGauge        metric.Float64Gauge
	credentialFailuresCounter    metric.Int64Counter
	credentialCooldownsCounter   metric.Int64Counter
	credentialExhaustionsCounter metric.Int64Counter

	// Provider
	providerRequestsCounter metric.Int64Counter
	providerErrorsCounter   metric.Int64Counter
	providerLatencyHist     metric.Float64Histogram
	providerRetriesCounter  metric.Int64Counter

	// AI Execution
	tokenCounter          metric.Int64Counter
	costCounter           metric.Float64Counter
	ttftDurationHist      metric.Float64Histogram
	activeRequestsCounter metric.Int64UpDownCounter
)

func init() {
	meter = otel.GetMeterProvider().Meter(MeterName)

	var err error
	requestCounter, err = meter.Int64Counter("prism_requests_total",
		metric.WithDescription("Total number of requests handled by Prism Gateway"),
	)
	if err != nil {
		log.Printf("otel: requestCounter: %v", err)
	}

	requestErrorCounter, err = meter.Int64Counter("prism_request_errors_total",
		metric.WithDescription("Total number of failed gateway requests"),
	)
	if err != nil {
		log.Printf("otel: requestErrorCounter: %v", err)
	}

	requestDurationHist, err = meter.Float64Histogram("prism_request_duration_seconds",
		metric.WithDescription("Request latency duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		log.Printf("otel: requestDurationHist: %v", err)
	}

	// Admission
	admissionAllowedCounter, _ = meter.Int64Counter("prism_admission_allowed_total")
	admissionDeniedCounter, _ = meter.Int64Counter("prism_admission_denied_total")
	budgetExceededCounter, _ = meter.Int64Counter("prism_budget_exceeded_total")
	quotaExceededCounter, _ = meter.Int64Counter("prism_quota_exceeded_total")

	// Routing
	routingDecisionsCounter, _ = meter.Int64Counter("prism_routing_decisions_total")
	routingFallbacksCounter, _ = meter.Int64Counter("prism_routing_fallbacks_total")

	// Credentials
	credentialHealthGauge, _ = meter.Float64Gauge("prism_credential_health_score")
	credentialFailuresCounter, _ = meter.Int64Counter("prism_credential_failures_total")
	credentialCooldownsCounter, _ = meter.Int64Counter("prism_credential_cooldowns_total")
	credentialExhaustionsCounter, _ = meter.Int64Counter("prism_credential_exhaustions_total")

	// Provider
	providerRequestsCounter, _ = meter.Int64Counter("prism_provider_requests_total")
	providerErrorsCounter, _ = meter.Int64Counter("prism_provider_errors_total")
	providerLatencyHist, _ = meter.Float64Histogram("prism_provider_latency_seconds", metric.WithUnit("s"))
	providerRetriesCounter, _ = meter.Int64Counter("prism_provider_retries_total")

	// AI Execution
	ttftDurationHist, _ = meter.Float64Histogram("prism_ttft_seconds", metric.WithUnit("s"))
	tokenCounter, _ = meter.Int64Counter("prism_token_usage_total")
	costCounter, _ = meter.Float64Counter("prism_cost_usd_total", metric.WithUnit("USD"))
	activeRequestsCounter, _ = meter.Int64UpDownCounter("prism_active_requests")
}

// Low-Cardinality helper to sanitize metric attributes
func sanitizeAttrs(model, provider, status, orgID string) metric.MeasurementOption {
	return metric.WithAttributes(
		attribute.String("model", model),
		attribute.String("provider", provider),
		attribute.String("status", status),
		attribute.String("org_id", orgID),
	)
}

// RecordRequestMetrics records completed request metrics.
func RecordRequestMetrics(ctx context.Context, model, provider, status, orgID string, latencySec float64, inputTokens, outputTokens int, costUSD float64) {
	attrs := sanitizeAttrs(model, provider, status, orgID)

	if requestCounter != nil {
		requestCounter.Add(ctx, 1, attrs)
	}
	if status != "200" && status != "200_ok" && requestErrorCounter != nil {
		requestErrorCounter.Add(ctx, 1, attrs)
	}
	if requestDurationHist != nil {
		requestDurationHist.Record(ctx, latencySec, attrs)
	}

	if tokenCounter != nil && (inputTokens > 0 || outputTokens > 0) {
		tokenCounter.Add(ctx, int64(inputTokens), metric.WithAttributes(
			attribute.String("type", "prompt"),
			attribute.String("model", model),
			attribute.String("provider", provider),
		))
		tokenCounter.Add(ctx, int64(outputTokens), metric.WithAttributes(
			attribute.String("type", "completion"),
			attribute.String("model", model),
			attribute.String("provider", provider),
		))
	}

	if costCounter != nil && costUSD > 0 {
		costCounter.Add(ctx, costUSD, attrs)
	}
}

// RecordAdmissionEvaluation records admission allowed/denied metrics.
func RecordAdmissionEvaluation(ctx context.Context, allowed bool, reason, orgID string) {
	attrs := metric.WithAttributes(attribute.String("org_id", orgID), attribute.String("reason", reason))
	if allowed {
		if admissionAllowedCounter != nil {
			admissionAllowedCounter.Add(ctx, 1, attrs)
		}
	} else {
		if admissionDeniedCounter != nil {
			admissionDeniedCounter.Add(ctx, 1, attrs)
		}
		if reason == "budget_exceeded" && budgetExceededCounter != nil {
			budgetExceededCounter.Add(ctx, 1, attrs)
		}
		if reason == "quota_exceeded" && quotaExceededCounter != nil {
			quotaExceededCounter.Add(ctx, 1, attrs)
		}
	}
}

// RecordRoutingDecision records routing and fallback decisions.
func RecordRoutingDecision(ctx context.Context, policy, strategy string, isFallback bool) {
	attrs := metric.WithAttributes(attribute.String("policy", policy), attribute.String("strategy", strategy))
	if routingDecisionsCounter != nil {
		routingDecisionsCounter.Add(ctx, 1, attrs)
	}
	if isFallback && routingFallbacksCounter != nil {
		routingFallbacksCounter.Add(ctx, 1, attrs)
	}
}

// RecordProviderAttempt records upstream provider HTTP request metrics.
func RecordProviderAttempt(ctx context.Context, provider, model string, statusCode int, latencySec float64, isRetry bool) {
	statusStr := "success"
	if statusCode >= 400 {
		statusStr = "error"
	}
	attrs := metric.WithAttributes(
		attribute.String("provider", provider),
		attribute.String("model", model),
		attribute.String("status", statusStr),
	)

	if providerRequestsCounter != nil {
		providerRequestsCounter.Add(ctx, 1, attrs)
	}
	if statusCode >= 400 && providerErrorsCounter != nil {
		providerErrorsCounter.Add(ctx, 1, attrs)
	}
	if providerLatencyHist != nil {
		providerLatencyHist.Record(ctx, latencySec, attrs)
	}
	if isRetry && providerRetriesCounter != nil {
		providerRetriesCounter.Add(ctx, 1, attrs)
	}
}

// RecordTTFT records Time To First Token for streaming requests.
func RecordTTFT(ctx context.Context, model, provider string, ttftSec float64) {
	if ttftDurationHist != nil && ttftSec > 0 {
		ttftDurationHist.Record(ctx, ttftSec, metric.WithAttributes(
			attribute.String("model", model),
			attribute.String("provider", provider),
		))
	}
}

// IncActiveRequests increments or decrements active requests count.
func IncActiveRequests(ctx context.Context, delta int64) {
	if activeRequestsCounter != nil {
		activeRequestsCounter.Add(ctx, delta)
	}
}

// RecordCredentialHealth records credential health score gauge.
func RecordCredentialHealth(ctx context.Context, credentialID, provider string, score float64) {
	if credentialHealthGauge != nil {
		credentialHealthGauge.Record(ctx, score, metric.WithAttributes(
			attribute.String("credential_id", credentialID),
			attribute.String("provider", provider),
		))
	}
}

// CredentialEventType enumerates credential lifecycle events.
const (
	CredentialEventFailure    = "failure"
	CredentialEventCooldown   = "cooldown"
	CredentialEventExhaustion = "exhaustion"
)

// RecordCredentialEvent records credential failure, cooldown, or exhaustion events.
func RecordCredentialEvent(ctx context.Context, eventType, credentialID, provider string) {
	var counter metric.Int64Counter
	switch eventType {
	case CredentialEventFailure:
		counter = credentialFailuresCounter
	case CredentialEventCooldown:
		counter = credentialCooldownsCounter
	case CredentialEventExhaustion:
		counter = credentialExhaustionsCounter
	default:
		return
	}
	if counter != nil {
		counter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("credential_id", credentialID),
			attribute.String("provider", provider),
		))
	}
}
