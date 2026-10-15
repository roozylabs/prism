package proxy

import (
	"context"
	"sync"
	"time"

	"github.com/roozylabs/prism/internal/repository"
)

// DefaultProviderHealthScore is applied when a provider has no observed
// traffic in the aggregation window (benefit of the doubt).
const DefaultProviderHealthScore = 0.95

// ComputeHealthScore returns a 0..1 provider health score from observed traffic:
// 0.7*successRate + 0.3*(1 - min(avgLatencyMs/5000, 1)), clamped to [0, 1].
// samples <= 0 → DefaultProviderHealthScore regardless of other inputs.
func ComputeHealthScore(successRate, avgLatencyMs float64, samples int64) float64 {
	if samples <= 0 {
		return DefaultProviderHealthScore
	}
	latencyScore := 1.0 - min(avgLatencyMs/5000.0, 1.0)
	score := 0.7*successRate + 0.3*latencyScore
	return min(max(score, 0.0), 1.0)
}

// minTenantHealthSamples is the number of requests a tenant needs on a
// provider in the aggregation window before its own score replaces the
// global one.
const minTenantHealthSamples = 10

// providerHealthSnapshot holds the scores computed from one fetch.
//   - global: per provider, counting only provider-side failures (5xx, no
//     status), so one tenant's bad credentials do not mark a provider
//     unhealthy for everyone.
//   - tenants: per org and provider, counting every failure, for tenants
//     with at least minTenantHealthSamples requests on that provider.
type providerHealthSnapshot struct {
	global  map[string]float64
	tenants map[string]map[string]float64
}

func buildHealthSnapshot(stats []repository.ProviderHealthStats) *providerHealthSnapshot {
	type totals struct {
		samples  int64
		serverOK float64
		latency  float64
	}
	byProvider := make(map[string]*totals)
	snap := &providerHealthSnapshot{
		global:  make(map[string]float64),
		tenants: make(map[string]map[string]float64),
	}

	for _, st := range stats {
		t := byProvider[st.ProviderID]
		if t == nil {
			t = &totals{}
			byProvider[st.ProviderID] = t
		}
		n := float64(st.Samples)
		t.samples += st.Samples
		t.serverOK += st.ServerSuccessRate * n
		t.latency += st.AvgLatencyMs * n

		if st.OrgID != "" && st.Samples >= minTenantHealthSamples {
			if snap.tenants[st.OrgID] == nil {
				snap.tenants[st.OrgID] = make(map[string]float64)
			}
			snap.tenants[st.OrgID][st.ProviderID] = ComputeHealthScore(st.SuccessRate, st.AvgLatencyMs, st.Samples)
		}
	}
	for providerID, t := range byProvider {
		if t.samples > 0 {
			n := float64(t.samples)
			snap.global[providerID] = ComputeHealthScore(t.serverOK/n, t.latency/n, t.samples)
		}
	}
	return snap
}

// ProviderHealthStore caches provider health scores computed from
// request_logs, refreshed at most once per TTL.
type ProviderHealthStore struct {
	repo    *repository.RequestLogRepository
	ttl     time.Duration
	mu      sync.Mutex
	snap    *providerHealthSnapshot
	fetched time.Time
	fetch   func(ctx context.Context) (*providerHealthSnapshot, error)
}

func NewProviderHealthStore(repo *repository.RequestLogRepository, ttl time.Duration) *ProviderHealthStore {
	s := &ProviderHealthStore{repo: repo, ttl: ttl}
	s.fetch = s.fetchFromRepo
	return s
}

func (s *ProviderHealthStore) fetchFromRepo(ctx context.Context) (*providerHealthSnapshot, error) {
	stats, err := s.repo.GetProviderHealthStats(ctx)
	if err != nil {
		return nil, err
	}
	return buildHealthSnapshot(stats), nil
}

// current returns the cached snapshot; refetches when stale. On fetch error
// it keeps the last known snapshot (possibly nil), never panics.
func (s *ProviderHealthStore) current(ctx context.Context) *providerHealthSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.snap != nil && time.Since(s.fetched) < s.ttl {
		return s.snap
	}
	fetched, err := s.fetch(ctx)
	if err != nil || fetched == nil {
		return s.snap
	}
	s.snap = fetched
	s.fetched = time.Now()
	return s.snap
}

// Scores returns the global provider→score map (provider-side failures only).
// Providers without traffic are absent — callers treat missing keys as
// DefaultProviderHealthScore. The returned map must not be modified.
func (s *ProviderHealthStore) Scores(ctx context.Context) map[string]float64 {
	snap := s.current(ctx)
	if snap == nil {
		return nil
	}
	return snap.global
}

// ScoresFor returns provider scores as seen by one tenant: the tenant's own
// score where it has enough traffic on a provider, the global score otherwise.
// The returned map must not be modified.
func (s *ProviderHealthStore) ScoresFor(ctx context.Context, orgID string) map[string]float64 {
	snap := s.current(ctx)
	if snap == nil {
		return nil
	}
	own := snap.tenants[orgID]
	if orgID == "" || len(own) == 0 {
		return snap.global
	}
	merged := make(map[string]float64, len(snap.global)+len(own))
	for providerID, score := range snap.global {
		merged[providerID] = score
	}
	for providerID, score := range own {
		merged[providerID] = score
	}
	return merged
}
