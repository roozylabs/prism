package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

const seededAdminID = "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"

func TestSystemProvidersSurviveAdminDeletion(t *testing.T) {
	db := migratedDatabase(t)

	var providers, models int
	if err := db.QueryRow(`SELECT count(*), (SELECT count(*) FROM models) FROM providers WHERE user_id IS NULL`).Scan(&providers, &models); err != nil {
		t.Fatal(err)
	}
	if providers == 0 || models == 0 {
		t.Fatalf("want ownerless system providers with models, got %d providers, %d models", providers, models)
	}

	if _, err := db.Exec(`DELETE FROM "user" WHERE id = $1`, seededAdminID); err != nil {
		t.Fatal(err)
	}

	var after, modelsAfter int
	if err := db.QueryRow(`SELECT count(*), (SELECT count(*) FROM models) FROM providers WHERE user_id IS NULL`).Scan(&after, &modelsAfter); err != nil {
		t.Fatal(err)
	}
	if after != providers || modelsAfter != models {
		t.Fatalf("deleting the admin user removed system providers: %d->%d providers, %d->%d models", providers, after, models, modelsAfter)
	}
}

func TestSystemProvidersAreSharedAndReadOnly(t *testing.T) {
	db := migratedDatabase(t)
	ctx := context.Background()
	if _, err := db.Exec(`INSERT INTO "user" (id, name, email) VALUES ('alice', 'alice', 'alice@example.com')`); err != nil {
		t.Fatal(err)
	}
	r := NewProviderRepository(db)

	list, err := r.ListByUserID(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	var system string
	for _, p := range list {
		if p.UserID == "" {
			system = p.ID
		}
	}
	if system == "" {
		t.Fatal("alice does not see the shared system providers")
	}

	p, err := r.FindByID(ctx, system)
	if err != nil {
		t.Fatalf("find system provider: %v", err)
	}
	p.BaseURL = "https://attacker.example"
	p.UserID = "alice"
	if err := r.Update(ctx, p); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("alice updating a system provider: err = %v, want no rows changed", err)
	}
	if err := r.Delete(ctx, system, "alice"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("alice deleting a system provider: err = %v, want no rows changed", err)
	}
	if got, _ := r.FindByID(ctx, system); got == nil || got.BaseURL == "https://attacker.example" {
		t.Fatalf("system provider changed: %+v", got)
	}
}
