package repository

import (
	"database/sql"
	"testing"
)

// tenantFixture is what the repository tests share: two users, an
// organization for each, one enabled provider per user, and alice's
// gateway key. Rows are created with plain SQL so a repository under test
// never seeds its own input.
type tenantFixture struct {
	db                 *sql.DB
	orgA, orgB         string
	provAlice, provBob string
	keyAlice           string
}

func newTenantFixture(t *testing.T) *tenantFixture {
	t.Helper()
	db := migratedDatabase(t)
	f := &tenantFixture{db: db, orgA: "org_alice", orgB: "org_bob"}
	if _, err := db.Exec(`
		INSERT INTO "user" (id, name, email) VALUES ('alice', 'alice', 'alice@example.com'), ('bob', 'bob', 'bob@example.com');
		INSERT INTO organizations (id, name, slug) VALUES ('org_alice', 'Alice Org', 'alice-org'), ('org_bob', 'Bob Org', 'bob-org');`); err != nil {
		t.Fatal(err)
	}
	for _, p := range []struct {
		user, slug string
		id         *string
	}{
		{"alice", "alice-openai", &f.provAlice},
		{"bob", "bob-openai", &f.provBob},
	} {
		if err := db.QueryRow(`INSERT INTO providers (user_id, name, slug, base_url, type, enabled, routing_strategy)
			VALUES ($1, $2, $2, 'http://localhost', 'openai', true, 'round_robin') RETURNING id`, p.user, p.slug).Scan(p.id); err != nil {
			t.Fatalf("provider %s: %v", p.slug, err)
		}
	}
	if err := db.QueryRow(`INSERT INTO gateway_api_keys (user_id, org_id, name, key_hash, key_prefix, enabled, rate_limit)
		VALUES ('alice', 'org_alice', 'alice-key', 'hash-alice', 'prism_sk_alice', true, 100) RETURNING id`).Scan(&f.keyAlice); err != nil {
		t.Fatal(err)
	}
	return f
}

// mustExec runs a statement and fails the test on error.
func mustExec(t *testing.T, db *sql.DB, query string, args ...interface{}) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}
