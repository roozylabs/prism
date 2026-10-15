package workers

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"time"

	goredis "github.com/roozylabs/prism/internal/redis"
	"github.com/roozylabs/prism/internal/repository"
)

// Each user's spend this hour is compared with their spend in the same hour
// on previous days, the only baseline that accounts for daily patterns.
const (
	anomalyBaselineDays = 14
	anomalyMinSamples   = 3
	anomalyWarningZ     = 3
	anomalyCriticalZ    = 4
)

type anomalyStore interface {
	CurrentHourSpend(ctx context.Context) (map[string]float64, time.Time, error)
	SameHourBaselines(ctx context.Context, windowStart time.Time, days int) (map[string][]float64, error)
	Create(ctx context.Context, a *repository.CostAnomaly) (bool, error)
}

type anomalyPublisher interface {
	Publish(ctx context.Context, userID, eventType string, data interface{}) error
}

// AnomalyDetector records spikes in a user's hourly spend and tells that user.
type AnomalyDetector struct {
	anomalies anomalyStore
	publisher anomalyPublisher
}

func NewAnomalyDetector(anomalies *repository.CostAnomalyRepository, publisher *goredis.EventPublisher) *AnomalyDetector {
	d := &AnomalyDetector{anomalies: anomalies}
	if publisher != nil {
		d.publisher = publisher
	}
	return d
}

func (d *AnomalyDetector) Run(ctx context.Context) {
	current, windowStart, err := d.anomalies.CurrentHourSpend(ctx)
	if err != nil {
		log.Printf("[anomaly] fetch current hour: %v", err)
		return
	}
	if len(current) == 0 {
		return
	}
	baselines, err := d.anomalies.SameHourBaselines(ctx, windowStart, anomalyBaselineDays)
	if err != nil {
		log.Printf("[anomaly] fetch baselines: %v", err)
		return
	}

	for userID, spend := range current {
		historical := baselines[userID]
		severity, mean, stddev, zScore, ok := evaluateHourlySpend(spend, historical)
		if !ok {
			continue
		}

		details, _ := json.Marshal(map[string]interface{}{
			"hour_of_day": windowStart.Hour(),
			"days_used":   len(historical),
		})
		uid := userID
		inserted, err := d.anomalies.Create(ctx, &repository.CostAnomaly{
			UserID:         &uid,
			WindowStart:    windowStart,
			Metric:         "hourly_spend",
			Observed:       spend,
			BaselineMean:   &mean,
			BaselineStddev: &stddev,
			ZScore:         &zScore,
			Severity:       severity,
			Details:        details,
		})
		if err != nil {
			log.Printf("[anomaly] create for user %s: %v", userID, err)
			continue
		}
		if inserted && d.publisher != nil {
			_ = d.publisher.Publish(ctx, userID, "cost_anomaly", map[string]interface{}{
				"severity":    severity,
				"observed":    spend,
				"z_score":     math.Round(zScore*100) / 100,
				"hour_of_day": windowStart.Hour(),
			})
		}
	}
}

// evaluateHourlySpend reports whether spend is an anomaly against historical
// same-hour spend, and how severe. It needs a few days of history with some
// variation to judge.
func evaluateHourlySpend(spend float64, historical []float64) (severity string, mean, stddev, zScore float64, ok bool) {
	if spend <= 0 || len(historical) < anomalyMinSamples {
		return "", 0, 0, 0, false
	}
	mean, stddev = computeStats(historical)
	if stddev == 0 {
		return "", mean, stddev, 0, false
	}
	zScore = (spend - mean) / stddev
	switch {
	case zScore >= anomalyCriticalZ:
		severity = "critical"
	case zScore >= anomalyWarningZ:
		severity = "warning"
	default:
		return "", mean, stddev, zScore, false
	}
	return severity, mean, stddev, zScore, true
}

func computeStats(vals []float64) (mean, stddev float64) {
	n := float64(len(vals))
	for _, v := range vals {
		mean += v
	}
	mean /= n
	var variance float64
	for _, v := range vals {
		diff := v - mean
		variance += diff * diff
	}
	variance /= n
	stddev = math.Sqrt(variance)
	return
}
