package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/roozylabs/prism/internal/models"
)

func TestGatewayKeyLookupsListingAndCounts(t *testing.T) {
	f := newTenantFixture(t)
	ctx := context.Background()
	r := NewGatewayKeyRepository(f.db)

	org := f.orgA
	// Migrations may seed keys of their own; count ours on top of those.
	baseGPT, err := r.CountByAllowedModel(ctx, "gpt-4o")
	if err != nil {
		t.Fatal(err)
	}
	expires := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	bound := &models.GatewayAPIKey{UserID: "alice", OrgID: &org, ProviderID: &f.provAlice, Name: "prod", KeyHash: "hash-prod", KeyPrefix: "prism_sk_prod", Enabled: true, RateLimit: 50, AllowedModels: []string{"gpt-4o"}, ExpiresAt: &expires}
	disabled := &models.GatewayAPIKey{UserID: "alice", OrgID: &org, Name: "old", KeyHash: "hash-old", KeyPrefix: "prism_sk_old", Enabled: false, RateLimit: 10}
	bobs := &models.GatewayAPIKey{UserID: "bob", Name: "bob-prod", KeyHash: "hash-bobprod", KeyPrefix: "prism_sk_bobp", Enabled: true, AllowedModels: []string{"gpt-4o", "claude-3"}}
	for _, k := range []*models.GatewayAPIKey{bound, disabled, bobs} {
		if err := r.Create(ctx, k); err != nil {
			t.Fatalf("create %s: %v", k.Name, err)
		}
	}

	got, err := r.FindByKeyHash(ctx, "hash-prod")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != bound.ID || got.ProviderID == nil || *got.ProviderID != f.provAlice || got.OrgID == nil || *got.OrgID != org {
		t.Fatalf("found = %+v", got)
	}
	if len(got.AllowedModels) != 1 || got.ExpiresAt == nil || !got.ExpiresAt.UTC().Equal(expires) {
		t.Fatalf("allowed models / expiry not read back: %+v", got)
	}
	if _, err := r.FindByKeyHash(ctx, "hash-old"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("disabled key resolved by hash: %v", err)
	}
	if k, err := r.FindByKeyPrefix(ctx, "prism_sk_prod"); err != nil || k.ID != bound.ID {
		t.Fatalf("by prefix = %+v, %v", k, err)
	}
	if k, _ := r.FindByKeyHash(ctx, "hash-bobprod"); len(k.AllowedModels) != 2 {
		t.Fatalf("bob's key = %+v", k)
	}
	if k, _ := r.FindByKeyHash(ctx, "hash-alice"); k.AllowedModels == nil {
		t.Fatalf("nil allowed_models should scan as an empty list: %+v", k)
	}

	// Listing: the fixture key plus the two above belong to alice.
	keys, total, err := r.ListByUserIDWithFilter(ctx, "alice", "", 2, 0)
	if err != nil || total != 3 || len(keys) != 2 {
		t.Fatalf("page 1 = %d of %d, %v", len(keys), total, err)
	}
	keys, total, _ = r.ListByUserIDWithFilter(ctx, "alice", "", 2, 2)
	if total != 3 || len(keys) != 1 {
		t.Fatalf("page 2 = %d of %d", len(keys), total)
	}
	if _, total, _ = r.ListByUserIDWithFilter(ctx, "alice", "PROD", 10, 0); total != 1 {
		t.Fatalf("search by name: total %d", total)
	}
	if _, total, _ = r.ListByUserIDWithFilter(ctx, "alice", "prism_sk_old", 10, 0); total != 1 {
		t.Fatalf("search by prefix: total %d", total)
	}
	if _, total, _ = r.ListByUserIDWithFilter(ctx, "alice", "", 10, 0, f.orgB); total != 0 {
		t.Fatalf("org filter: total %d, want 0", total)
	}
	if all, err := r.ListByUserID(ctx, "bob"); err != nil || len(all) != 1 {
		t.Fatalf("bob's keys = %+v, %v", all, err)
	}

	bound.Name = "prod-renamed"
	bound.AllowedModels = []string{"gpt-4o", "o3"}
	bound.RateLimit = 5
	if err := r.Update(ctx, bound); err != nil {
		t.Fatal(err)
	}
	if err := r.IncrementUsage(ctx, bound.ID); err != nil {
		t.Fatal(err)
	}
	got, _ = r.FindByKeyHash(ctx, "hash-prod")
	if got.Name != "prod-renamed" || len(got.AllowedModels) != 2 || got.RateLimit != 5 || got.RequestCount != 1 || got.LastUsedAt == nil {
		t.Fatalf("update/usage not persisted: %+v", got)
	}

	if n, err := r.CountByProviderID(ctx, f.provAlice); err != nil || n != 1 {
		t.Fatalf("count by provider = %d, %v", n, err)
	}
	if n, _ := r.CountByAllowedModel(ctx, "gpt-4o"); n != baseGPT+2 {
		t.Fatalf("count by allowed model = %d, want %d (alice's and bob's)", n, baseGPT+2)
	}
	if n, _ := r.CountByAllowedModel(ctx, "o3"); n != 1 {
		t.Fatalf("count by allowed model o3 = %d", n)
	}

	if err := r.Delete(ctx, bound.ID, "bob"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("bob deleted alice's key: %v", err)
	}
	if err := r.Delete(ctx, bound.ID, "alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.FindByKeyHash(ctx, "hash-prod"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted key still resolves: %v", err)
	}
	if err := r.Delete(ctx, bound.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("second delete: %v", err)
	}
}
