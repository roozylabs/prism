package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/roozylabs/prism/internal/models"
)

func TestRoutingPolicyLookupsAreOwnerScoped(t *testing.T) {
	f := newTenantFixture(t)
	ctx := context.Background()
	r := NewRoutingPolicyRepository(f.db)

	balanced := &models.RoutingPolicy{UserID: "alice", Name: "balanced", Weights: map[string]float64{"quality": 0.5, "cost": 0.5}, Constraints: map[string]float64{"max_cost_per_request": 0.05}, Enabled: true, IsDefault: true}
	fast := &models.RoutingPolicy{UserID: "alice", Name: "fast", Weights: map[string]float64{"speed": 1}, Enabled: true}
	retired := &models.RoutingPolicy{UserID: "alice", Name: "retired", Weights: map[string]float64{}, Enabled: false}
	bobFast := &models.RoutingPolicy{UserID: "bob", Name: "fast", Weights: map[string]float64{"speed": 0.9}, Enabled: true}
	for _, p := range []*models.RoutingPolicy{balanced, fast, retired, bobFast} {
		if err := r.Create(ctx, p); err != nil {
			t.Fatalf("create %s/%s: %v", p.UserID, p.Name, err)
		}
	}
	if err := r.Create(ctx, &models.RoutingPolicy{UserID: "alice", Name: "fast", Enabled: true}); !IsUniqueViolation(err) {
		t.Fatalf("duplicate name: err = %v, want unique violation", err)
	}

	list, err := r.ListByUserID(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 || list[0].Name != "balanced" || list[1].Name != "fast" || list[2].Name != "retired" {
		t.Fatalf("alice policies = %+v, want default, then enabled, then by name", list)
	}
	if list[0].Weights["quality"] != 0.5 || list[0].Constraints["max_cost_per_request"] != 0.05 {
		t.Fatalf("JSON columns not decoded: %+v", list[0])
	}

	got, err := r.FindByName(ctx, "fast", "bob")
	if err != nil || got.ID != bobFast.ID || got.Weights["speed"] != 0.9 {
		t.Fatalf("bob's fast = %+v, %v", got, err)
	}
	if _, err := r.FindByName(ctx, "balanced", "bob"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("bob found alice's policy by name: %v", err)
	}
	if _, err := r.FindByID(ctx, balanced.ID, "bob"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("bob found alice's policy by id: %v", err)
	}

	def, err := r.FindByDefault(ctx, "alice")
	if err != nil || def.ID != balanced.ID {
		t.Fatalf("default = %+v, %v", def, err)
	}
	// Without a flagged default the first enabled policy by name is used.
	def, err = r.FindByDefault(ctx, "bob")
	if err != nil || def.ID != bobFast.ID {
		t.Fatalf("bob's fallback default = %+v, %v", def, err)
	}

	// Updating a policy to default clears the previous default.
	fast.IsDefault = true
	fast.Weights["speed"] = 0.7
	if err := r.Update(ctx, fast); err != nil {
		t.Fatal(err)
	}
	old, _ := r.FindByID(ctx, balanced.ID, "alice")
	now, _ := r.FindByID(ctx, fast.ID, "alice")
	if old.IsDefault || !now.IsDefault || now.Weights["speed"] != 0.7 {
		t.Fatalf("default not moved: old=%+v new=%+v", old, now)
	}

	if err := r.Delete(ctx, fast.ID, "bob"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("bob deleted alice's policy: %v", err)
	}
	if err := r.Delete(ctx, fast.ID, "alice"); err != nil {
		t.Fatal(err)
	}
	if list, _ := r.ListByUserID(ctx, "alice"); len(list) != 2 {
		t.Fatalf("after delete: %+v", list)
	}
}
