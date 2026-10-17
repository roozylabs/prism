package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/roozylabs/prism/internal/models"
)

func TestGatewayKeyDeleteReturnsHash(t *testing.T) {
	db := migratedDatabase(t)
	ctx := context.Background()
	if _, err := db.Exec(`INSERT INTO "user" (id, name, email) VALUES ('u1', 'u1', 'u1@example.com'), ('u2', 'u2', 'u2@example.com')`); err != nil {
		t.Fatal(err)
	}
	r := NewGatewayKeyRepository(db)
	key := &models.GatewayAPIKey{UserID: "u1", Name: "k", KeyHash: "hash-1", KeyPrefix: "prism_sk_abc", Enabled: true, RateLimit: 100}
	if err := r.Create(ctx, key); err != nil {
		t.Fatal(err)
	}

	if _, err := r.Delete(ctx, key.ID, "u2"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("another user's delete: err %v, want sql.ErrNoRows", err)
	}
	hash, err := r.Delete(ctx, key.ID, "u1")
	if err != nil || hash != "hash-1" {
		t.Fatalf("delete: hash %q err %v, want the key's hash", hash, err)
	}
	if _, err := r.Delete(ctx, key.ID, "u1"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("second delete: err %v, want sql.ErrNoRows", err)
	}
	if _, err := r.FindByKeyHash(ctx, "hash-1"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted key still found: err %v", err)
	}
}
