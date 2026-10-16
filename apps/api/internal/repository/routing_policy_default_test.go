package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestSetDefaultKeepsDefaultOnUnknownPolicy(t *testing.T) {
	db := migratedDatabase(t)
	ctx := context.Background()
	if _, err := db.Exec(`
		INSERT INTO "user" (id, name, email) VALUES ('alice', 'alice', 'alice@example.com'), ('bob', 'bob', 'bob@example.com');
		INSERT INTO routing_policies (id, user_id, name, is_default, enabled) VALUES
		  ('00000000-0000-0000-0000-00000000000a', 'alice', 'main', true, true),
		  ('00000000-0000-0000-0000-00000000000b', 'alice', 'paused', false, false),
		  ('00000000-0000-0000-0000-00000000000c', 'bob', 'bobs', true, true);
		INSERT INTO budgets (id, user_id, name) VALUES ('00000000-0000-0000-0000-0000000000b1', 'bob', 'b');`); err != nil {
		t.Fatal(err)
	}
	policies := NewRoutingPolicyRepository(db)

	// Bob's id used to clear Alice's default and still answer 200.
	if err := policies.SetDefault(ctx, "00000000-0000-0000-0000-00000000000c", "alice"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("foreign policy: err = %v, want sql.ErrNoRows", err)
	}
	if def, err := policies.FindByDefault(ctx, "alice"); err != nil || def.Name != "main" {
		t.Fatalf("alice's default after the failed call: %+v, %v", def, err)
	}

	// A disabled policy used to vanish from the list and could not be
	// enabled again.
	list, err := policies.ListByUserID(ctx, "alice")
	if err != nil || len(list) != 2 {
		t.Fatalf("alice lists %d policies, %v; want both, the disabled one included", len(list), err)
	}
	if err := policies.SetDefault(ctx, "00000000-0000-0000-0000-00000000000b", "alice"); err != nil {
		t.Fatal(err)
	}
	if def, err := policies.FindByDefault(ctx, "alice"); err != nil || def.Name != "paused" || !def.Enabled {
		t.Fatalf("new default: %+v, %v", def, err)
	}

	// Deleting someone else's row used to report success.
	if err := NewBudgetRepository(db).Delete(ctx, "00000000-0000-0000-0000-0000000000b1", "alice"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleting bob's budget as alice: err = %v, want sql.ErrNoRows", err)
	}
}
