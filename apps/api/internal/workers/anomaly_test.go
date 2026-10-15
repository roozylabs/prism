package workers

import (
	"context"
	"testing"
	"time"

	"github.com/roozylabs/prism/internal/repository"
)

func TestEvaluateHourlySpend(t *testing.T) {
	history := []float64{1, 1.2, 0.8, 1.1, 0.9} // mean 1, stddev ~0.14

	for name, tc := range map[string]struct {
		spend    float64
		history  []float64
		severity string
		ok       bool
	}{
		"normal hour":           {1.1, history, "", false},
		"warning spike":         {1.5, history, "warning", true},
		"critical spike":        {5, history, "critical", true},
		"too little history":    {5, []float64{1, 2}, "", false},
		"no variation to judge": {5, []float64{1, 1, 1}, "", false},
		"no spend":              {0, history, "", false},
	} {
		t.Run(name, func(t *testing.T) {
			severity, _, _, _, ok := evaluateHourlySpend(tc.spend, tc.history)
			if ok != tc.ok || severity != tc.severity {
				t.Fatalf("got %q, %v; want %q, %v", severity, ok, tc.severity, tc.ok)
			}
		})
	}
}

type fakeAnomalyStore struct {
	current   map[string]float64
	baselines map[string][]float64
	existing  map[string]bool // users who already have this hour's anomaly
	created   []repository.CostAnomaly
}

func (f *fakeAnomalyStore) CurrentHourSpend(context.Context) (map[string]float64, time.Time, error) {
	return f.current, time.Date(2026, 10, 15, 14, 0, 0, 0, time.UTC), nil
}

func (f *fakeAnomalyStore) SameHourBaselines(context.Context, time.Time, int) (map[string][]float64, error) {
	return f.baselines, nil
}

func (f *fakeAnomalyStore) Create(_ context.Context, a *repository.CostAnomaly) (bool, error) {
	f.created = append(f.created, *a)
	return !f.existing[*a.UserID], nil
}

type fakeAnomalyPublisher struct{ to []string }

func (f *fakeAnomalyPublisher) Publish(_ context.Context, userID, _ string, _ interface{}) error {
	f.to = append(f.to, userID)
	return nil
}

func TestAnomalyDetectorWorksPerUser(t *testing.T) {
	steady := []float64{1, 1.2, 0.8, 1.1, 0.9}
	store := &fakeAnomalyStore{
		current: map[string]float64{
			"alice": 5,   // spike against her own history
			"bob":   1.1, // normal for bob
			"carol": 50,  // big, but no history to compare with
		},
		baselines: map[string][]float64{"alice": steady, "bob": steady},
	}
	pub := &fakeAnomalyPublisher{}
	d := &AnomalyDetector{anomalies: store, publisher: pub}

	d.Run(context.Background())

	if len(store.created) != 1 || *store.created[0].UserID != "alice" || store.created[0].Severity != "critical" {
		t.Fatalf("created %+v, want one critical anomaly for alice", store.created)
	}
	if len(pub.to) != 1 || pub.to[0] != "alice" {
		t.Fatalf("published to %v, want only alice", pub.to)
	}

	// A later run in the same hour finds the anomaly already stored and does
	// not notify again.
	store.existing = map[string]bool{"alice": true}
	pub.to = nil
	d.Run(context.Background())
	if len(pub.to) != 0 {
		t.Fatalf("published again to %v", pub.to)
	}
}
