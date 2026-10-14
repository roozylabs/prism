package redis

import (
	"encoding/json"
	"testing"
)

func sampleJSON(t *testing.T, s LatencySample) string {
	t.Helper()
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestAggregateSamples_FailuresLowerSuccessRateNotP95(t *testing.T) {
	var items []string
	for i := 0; i < 8; i++ {
		items = append(items, sampleJSON(t, LatencySample{TTFTMs: 400, LatencyMs: 900, Success: true}))
	}
	// Failed attempts are recorded with TTFT 0.
	for i := 0; i < 2; i++ {
		items = append(items, sampleJSON(t, LatencySample{TTFTMs: 0, LatencyMs: 30000, Success: false}))
	}

	m := aggregateSamples("gpt-4o", items)
	if m.SampleCount != 10 {
		t.Fatalf("expected 10 samples, got %d", m.SampleCount)
	}
	if m.SuccessRate != 0.8 {
		t.Errorf("expected success rate 0.8, got %v", m.SuccessRate)
	}
	if m.P95TTFTMs != 400 {
		t.Errorf("expected p95 TTFT 400 (failures excluded), got %v", m.P95TTFTMs)
	}
}
