package workers

import (
	"context"
	"errors"
	"testing"

	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/repository"
)

type fakeExhaustedStore struct {
	exhausted []repository.ExhaustedCredential
	listErr   error
	recovered map[string]string
}

func (f *fakeExhaustedStore) ListExhausted(context.Context) ([]repository.ExhaustedCredential, error) {
	return f.exhausted, f.listErr
}

func (f *fakeExhaustedStore) RecoverExhausted(_ context.Context, id, status string) (bool, error) {
	if f.recovered == nil {
		f.recovered = map[string]string{}
	}
	f.recovered[id] = status
	return true, nil
}

type fakeCooldown struct {
	cooling map[string]bool
	err     error
}

func (f fakeCooldown) IsCoolingDown(_ context.Context, id string) (bool, error) {
	return f.cooling[id], f.err
}

func TestCredentialRecovery_RecoversOnlyExpiredCooldowns(t *testing.T) {
	store := &fakeExhaustedStore{exhausted: []repository.ExhaustedCredential{
		{ID: "expired-healthy", HealthScore: 95},
		{ID: "expired-degraded", HealthScore: 40},
		{ID: "still-cooling", HealthScore: 95},
	}}
	w := NewCredentialRecoveryWorker(store, fakeCooldown{cooling: map[string]bool{"still-cooling": true}})

	w.Run(context.Background())

	if got := store.recovered["expired-healthy"]; got != models.CredentialStatusHealthy {
		t.Errorf("expired-healthy: expected %q, got %q", models.CredentialStatusHealthy, got)
	}
	if got := store.recovered["expired-degraded"]; got != models.CredentialStatusDegraded {
		t.Errorf("expired-degraded: expected %q, got %q", models.CredentialStatusDegraded, got)
	}
	if _, touched := store.recovered["still-cooling"]; touched {
		t.Error("a credential still in its cooldown must stay exhausted")
	}
}

func TestCredentialRecovery_RedisErrorLeavesCredentialsExhausted(t *testing.T) {
	store := &fakeExhaustedStore{exhausted: []repository.ExhaustedCredential{{ID: "c1", HealthScore: 95}}}
	w := NewCredentialRecoveryWorker(store, fakeCooldown{err: errors.New("redis down")})

	w.Run(context.Background())

	if len(store.recovered) != 0 {
		t.Errorf("no credential should be recovered while Redis is unavailable, got %v", store.recovered)
	}
}

func TestCredentialRecovery_ListErrorIsHarmless(t *testing.T) {
	store := &fakeExhaustedStore{listErr: errors.New("db down")}
	NewCredentialRecoveryWorker(store, fakeCooldown{}).Run(context.Background())
	if len(store.recovered) != 0 {
		t.Errorf("nothing should be recovered on a list error, got %v", store.recovered)
	}
}
