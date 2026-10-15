package proxy

import (
	"context"
	"errors"
	"github.com/roozylabs/prism/internal/repository"
	"testing"
	"time"
)

func TestComputeHealthScore_NoSamples(t *testing.T) {
	cases := []struct {
		successRate  float64
		avgLatencyMs float64
	}{
		{0.0, 0.0},
		{1.0, 0.0},
		{0.42, 9000},
		{-1.0, -5000},
	}
	for _, tc := range cases {
		got := ComputeHealthScore(tc.successRate, tc.avgLatencyMs, 0)
		if got != 0.95 {
			t.Errorf("ComputeHealthScore(%v, %v, 0) = %v, want 0.95", tc.successRate, tc.avgLatencyMs, got)
		}
	}
}

func TestComputeHealthScore_Perfect(t *testing.T) {
	got := ComputeHealthScore(1.0, 0, 100)
	if got != 1.0 {
		t.Errorf("ComputeHealthScore(1.0, 0, 100) = %v, want 1.0", got)
	}
}

func TestComputeHealthScore_HalfErrorsLowLatency(t *testing.T) {
	got := ComputeHealthScore(0.5, 0, 10)
	want := 0.65
	const eps = 1e-9
	diff := got - want
	if diff < -eps || diff > eps {
		t.Errorf("ComputeHealthScore(0.5, 0, 10) = %v, want %v", got, want)
	}
}

func TestComputeHealthScore_LatencyCapped(t *testing.T) {
	got := ComputeHealthScore(1.0, 10000, 50)
	want := 0.70
	const eps = 1e-9
	diff := got - want
	if diff < -eps || diff > eps {
		t.Errorf("ComputeHealthScore(1.0, 10000, 50) = %v, want %v", got, want)
	}
}

func TestComputeHealthScore_ClampedToZeroOne(t *testing.T) {
	low := ComputeHealthScore(-1.0, 10000, 25)
	if low != 0.0 {
		t.Errorf("negative inputs produced %v, want clamped 0.0", low)
	}
	high := ComputeHealthScore(5.0, -60000, 25)
	if high != 1.0 {
		t.Errorf("overshoot inputs produced %v, want clamped 1.0", high)
	}
}

// newTestHealthStore builds a store whose fetch returns global scores only.
func newTestHealthStore(ttl time.Duration, fetch func(ctx context.Context) (map[string]float64, error)) *ProviderHealthStore {
	return &ProviderHealthStore{
		ttl: ttl,
		fetch: func(ctx context.Context) (*providerHealthSnapshot, error) {
			global, err := fetch(ctx)
			if err != nil {
				return nil, err
			}
			return &providerHealthSnapshot{global: global}, nil
		},
	}
}

func TestHealthStore_FreshCacheSkipsFetch(t *testing.T) {
	ctx := context.Background()
	fetches := 0
	s := newTestHealthStore(time.Minute, func(ctx context.Context) (map[string]float64, error) {
		fetches++
		return map[string]float64{"p1": 0.8}, nil
	})

	first := s.Scores(ctx)
	second := s.Scores(ctx)

	if fetches != 1 {
		t.Errorf("expected 1 fetch within TTL, got %d", fetches)
	}
	if first["p1"] != 0.8 || second["p1"] != 0.8 {
		t.Errorf("unexpected cached scores: %v / %v", first, second)
	}
}

func TestHealthStore_StaleCacheRefetches(t *testing.T) {
	ctx := context.Background()
	fetches := 0
	s := newTestHealthStore(0, func(ctx context.Context) (map[string]float64, error) {
		fetches++
		if fetches == 1 {
			return map[string]float64{"p1": 0.8}, nil
		}
		return map[string]float64{"p1": 0.2}, nil
	})

	first := s.Scores(ctx)["p1"]
	second := s.Scores(ctx)["p1"]

	if fetches != 2 {
		t.Errorf("expected refetch when stale, got %d fetches", fetches)
	}
	if first != 0.8 || second != 0.2 {
		t.Errorf("expected refreshed score, got %v then %v", first, second)
	}
}

func TestHealthStore_ErrorKeepsLastKnownScores(t *testing.T) {
	ctx := context.Background()
	call := 0
	s := newTestHealthStore(0, func(ctx context.Context) (map[string]float64, error) {
		call++
		if call == 1 {
			return map[string]float64{"p1": 0.9}, nil
		}
		return nil, errors.New("db down")
	})

	first := s.Scores(ctx)
	second := s.Scores(ctx)

	if first["p1"] != 0.9 || second["p1"] != 0.9 {
		t.Errorf("expected last known scores on error, got %v then %v", first, second)
	}
}

func TestHealthStore_ConcurrentScoresSafe(t *testing.T) {
	ctx := context.Background()
	s := newTestHealthStore(time.Millisecond, func(ctx context.Context) (map[string]float64, error) {
		return map[string]float64{"p1": 0.5}, nil
	})

	done := make(chan struct{})
	for i := 0; i < 16; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 50; j++ {
				_ = s.Scores(ctx)
			}
		}()
	}
	for i := 0; i < 16; i++ {
		<-done
	}
}

func TestBuildHealthSnapshot_TenantCredentialErrorsStayTenantLocal(t *testing.T) {
	// org_a's own key is invalid: every request gets a 401 (a 4xx, so its
	// provider-side success rate is still 1.0). org_b works fine.
	snap := buildHealthSnapshot([]repository.ProviderHealthStats{
		{OrgID: "org_a", ProviderID: "p1", SuccessRate: 0, ServerSuccessRate: 1, AvgLatencyMs: 200, Samples: 50},
		{OrgID: "org_b", ProviderID: "p1", SuccessRate: 1, ServerSuccessRate: 1, AvgLatencyMs: 200, Samples: 50},
	})

	if g := snap.global["p1"]; g < 0.95 {
		t.Errorf("global score should ignore tenant 4xx errors, got %.3f", g)
	}
	if a := snap.tenants["org_a"]["p1"]; a > 0.5 {
		t.Errorf("org_a should see its own failures, got %.3f", a)
	}
	if b := snap.tenants["org_b"]["p1"]; b < 0.95 {
		t.Errorf("org_b should not be affected by org_a, got %.3f", b)
	}
}

func TestBuildHealthSnapshot_ProviderOutageIsGlobal(t *testing.T) {
	// org_a hits a provider outage (5xx); org_c has no traffic of its own.
	snap := buildHealthSnapshot([]repository.ProviderHealthStats{
		{OrgID: "org_a", ProviderID: "p1", SuccessRate: 0.2, ServerSuccessRate: 0.2, AvgLatencyMs: 200, Samples: 50},
	})
	s := &ProviderHealthStore{ttl: time.Minute, fetch: func(context.Context) (*providerHealthSnapshot, error) { return snap, nil }}

	if got := s.ScoresFor(context.Background(), "org_c")["p1"]; got > 0.5 {
		t.Errorf("a tenant without traffic should still see the provider outage, got %.3f", got)
	}
}

func TestBuildHealthSnapshot_FewSamplesUseGlobal(t *testing.T) {
	snap := buildHealthSnapshot([]repository.ProviderHealthStats{
		{OrgID: "org_a", ProviderID: "p1", SuccessRate: 0, ServerSuccessRate: 1, AvgLatencyMs: 200, Samples: minTenantHealthSamples - 1},
		{OrgID: "org_b", ProviderID: "p1", SuccessRate: 1, ServerSuccessRate: 1, AvgLatencyMs: 200, Samples: 100},
	})
	s := &ProviderHealthStore{ttl: time.Minute, fetch: func(context.Context) (*providerHealthSnapshot, error) { return snap, nil }}

	if _, ok := snap.tenants["org_a"]; ok {
		t.Error("org_a has too few samples for a tenant score")
	}
	if got := s.ScoresFor(context.Background(), "org_a")["p1"]; got < 0.95 {
		t.Errorf("org_a should fall back to the global score, got %.3f", got)
	}
}

func TestScoresFor_DoesNotMutateGlobal(t *testing.T) {
	snap := buildHealthSnapshot([]repository.ProviderHealthStats{
		{OrgID: "org_a", ProviderID: "p1", SuccessRate: 0, ServerSuccessRate: 1, AvgLatencyMs: 200, Samples: 50},
		{OrgID: "org_b", ProviderID: "p2", SuccessRate: 1, ServerSuccessRate: 1, AvgLatencyMs: 200, Samples: 50},
	})
	s := &ProviderHealthStore{ttl: time.Minute, fetch: func(context.Context) (*providerHealthSnapshot, error) { return snap, nil }}
	ctx := context.Background()

	_ = s.ScoresFor(ctx, "org_a")
	if got := s.Scores(ctx)["p1"]; got < 0.95 {
		t.Errorf("tenant overlay leaked into the global scores, got %.3f", got)
	}
	if got := s.ScoresFor(ctx, "")["p1"]; got < 0.95 {
		t.Errorf("an empty org should get global scores, got %.3f", got)
	}
}
