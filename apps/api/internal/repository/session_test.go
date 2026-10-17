package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/utils"
)

// The session table used to hold the bearer token itself; a database read
// or a backup handed out every live session.
func TestSessionTokensAreStoredHashed(t *testing.T) {
	db := migratedDatabase(t)
	ctx := context.Background()
	if _, err := db.Exec(`INSERT INTO "user" (id, name, email) VALUES ('alice', 'alice', 'alice@example.com')`); err != nil {
		t.Fatal(err)
	}
	r := NewSessionRepository(db)
	const raw = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if err := r.Create(ctx, &models.Session{Token: raw, UserID: "alice", ExpiresAt: time.Now().Add(time.Hour), IPAddress: "203.0.113.7"}); err != nil {
		t.Fatal(err)
	}

	var stored string
	if err := db.QueryRow(`SELECT token FROM session WHERE user_id = 'alice'`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == raw || stored != utils.HashSHA256(raw) {
		t.Fatalf("stored token = %q, want the SHA-256 of the bearer value", stored)
	}

	// The client keeps presenting the raw token; the stored hash is not a
	// valid bearer value.
	if s, err := r.FindValidByToken(ctx, raw); err != nil || s.UserID != "alice" || s.IPAddress != "203.0.113.7" {
		t.Fatalf("lookup by bearer token: %+v, %v", s, err)
	}
	if _, err := r.FindByToken(ctx, stored); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("the stored hash must not work as a token: %v", err)
	}

	if err := r.Delete(ctx, raw); err != nil {
		t.Fatal(err)
	}
	if _, err := r.FindByToken(ctx, raw); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("session still found after delete: %v", err)
	}
}

// Migration 109 hashes the rows that existed before, so cookies issued
// earlier keep working after the upgrade.
func TestExistingSessionRowsWereHashedInPlace(t *testing.T) {
	db := migratedDatabase(t)
	if _, err := db.Exec(`INSERT INTO "user" (id, name, email) VALUES ('alice', 'alice', 'alice@example.com')`); err != nil {
		t.Fatal(err)
	}
	// Simulate a pre-109 row, then apply 109's statement to it.
	if _, err := db.Exec(`INSERT INTO session (id, expires_at, token, user_id) VALUES ('s1', NOW() + interval '1 hour', 'legacy-bearer-token', 'alice')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE session SET token = encode(sha256(convert_to(token, 'UTF8')), 'hex') WHERE id = 's1'`); err != nil {
		t.Fatal(err)
	}
	if s, err := NewSessionRepository(db).FindValidByToken(context.Background(), "legacy-bearer-token"); err != nil || s.ID != "s1" {
		t.Fatalf("legacy session not found by its bearer token after hashing: %+v, %v", s, err)
	}
}
