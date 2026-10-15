package workers

import (
	"context"
	"log"

	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	"github.com/roozylabs/prism/internal/repository"
)

type exhaustedCredentialStore interface {
	ListExhausted(ctx context.Context) ([]repository.ExhaustedCredential, error)
	RecoverExhausted(ctx context.Context, credentialID, status string) (bool, error)
}

type cooldownChecker interface {
	IsCoolingDown(ctx context.Context, credentialID string) (bool, error)
}

// CredentialRecoveryWorker clears the exhausted status of credentials whose
// cooldown has expired. Exhaustion always sets a cooldown that lasts until the
// provider's quota resets (UTC midnight for daily quotas, otherwise the
// Retry-After or reset header, 300s by default), so an expired cooldown means
// the credential can be used again.
type CredentialRecoveryWorker struct {
	creds    exhaustedCredentialStore
	cooldown cooldownChecker
}

func NewCredentialRecoveryWorker(creds exhaustedCredentialStore, cooldown cooldownChecker) *CredentialRecoveryWorker {
	return &CredentialRecoveryWorker{creds: creds, cooldown: cooldown}
}

func (w *CredentialRecoveryWorker) Run(ctx context.Context) {
	exhausted, err := w.creds.ListExhausted(ctx)
	if err != nil {
		log.Printf("[credential-recovery] list exhausted credentials: %v", err)
		return
	}
	for _, c := range exhausted {
		cooling, err := w.cooldown.IsCoolingDown(ctx, c.ID)
		if err != nil || cooling {
			// Still in its quota window, or Redis is unavailable: leave it.
			continue
		}
		status := proxy.DetermineCredentialStatus(true, false, false, c.HealthScore)
		if recovered, err := w.creds.RecoverExhausted(ctx, c.ID, status); err != nil {
			log.Printf("[credential-recovery] recover %s: %v", c.ID, err)
		} else if recovered {
			log.Printf("[credential-recovery] %s: %s -> %s", c.ID, models.CredentialStatusExhausted, status)
		}
	}
}
