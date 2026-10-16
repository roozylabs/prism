package database

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/lib/pq"
)

const migrationsDir = "../../migrations"

// TestMigrationsFromEmptyDatabase applies every migration to a new, empty
// database and then rolls them all back, so a broken chain fails CI instead
// of the first fresh install.
func TestMigrationsFromEmptyDatabase(t *testing.T) {
	dbURL := freshDatabase(t)

	if err := RunMigrations(dbURL, migrationsDir); err != nil {
		t.Fatalf("up: %v", err)
	}

	m, err := migrate.New("file://"+migrationsDir, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = m.Close() }()

	version, dirty, err := m.Version()
	if err != nil {
		t.Fatal(err)
	}
	if want := latestMigration(t); version != want || dirty {
		t.Fatalf("after up: version %d dirty=%v, want version %d clean", version, dirty, want)
	}

	// The role/plan matrix accounts share a published password; they belong
	// in cmd/seed for development, never in a migrated database.
	var seeded int
	if err := open(t, dbURL).QueryRow(`SELECT count(*) FROM "user" WHERE email LIKE '%@prism.local'`).Scan(&seeded); err != nil {
		t.Fatal(err)
	}
	if seeded > 0 {
		t.Fatalf("migrations leave %d @prism.local seed accounts behind", seeded)
	}

	// Every foreign key needs an index that starts with its column, or
	// cascades and tenant filters scan the whole table.
	if unindexed := queryStrings(t, dbURL, `
		SELECT DISTINCT c.conrelid::regclass || '.' || a.attname
		FROM pg_constraint c
		JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = c.conkey[1]
		WHERE c.contype = 'f' AND array_length(c.conkey, 1) = 1
		  AND NOT EXISTS (SELECT 1 FROM pg_index i WHERE i.indrelid = c.conrelid AND i.indkey[0] = c.conkey[1])`); len(unindexed) > 0 {
		t.Fatalf("foreign keys without an index: %v", unindexed)
	}
	// "user".id is text; every column holding a user id must match it.
	if mismatched := queryStrings(t, dbURL, `
		SELECT table_name || '.' || column_name || ' ' || data_type
		FROM information_schema.columns
		WHERE table_schema = 'public' AND column_name IN ('user_id', 'actor_id') AND data_type <> 'text'`); len(mismatched) > 0 {
		t.Fatalf("user id columns that are not text: %v", mismatched)
	}
	// One name for the column that points at organizations(id), and no views
	// left behind by renames: a view freezes its column list when created.
	if leftovers := queryStrings(t, dbURL, `
		SELECT table_name || '.' || column_name FROM information_schema.columns
		WHERE table_schema = 'public' AND column_name = 'organization_id'
		UNION ALL
		SELECT 'view ' || table_name FROM information_schema.views WHERE table_schema = 'public'`); len(leftovers) > 0 {
		t.Fatalf("use org_id and no views: %v", leftovers)
	}
	// Two indexes on the same columns double the cost of every write.
	if duplicates := queryStrings(t, dbURL, `
		SELECT string_agg(indexrelid::regclass::text, ' = ')
		FROM pg_index
		GROUP BY indrelid, indkey, indexprs::text, indpred::text
		HAVING count(*) > 1`); len(duplicates) > 0 {
		t.Fatalf("duplicate indexes: %v", duplicates)
	}

	if err := m.Down(); err != nil {
		t.Fatalf("down: %v", err)
	}

	db := open(t, dbURL)
	var left []string
	rows, err := db.Query(`SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name <> 'schema_migrations'`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		left = append(left, name)
	}
	if len(left) > 0 {
		t.Fatalf("down left tables or views behind: %v", left)
	}
}

// freshDatabase creates an empty database next to DATABASE_URL's and returns
// its URL. The database is dropped when the test ends.
func freshDatabase(t *testing.T) string {
	t.Helper()
	base := os.Getenv("DATABASE_URL")
	if base == "" {
		t.Skip("DATABASE_URL not set")
	}
	admin := open(t, base)
	if err := admin.Ping(); err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}

	name := fmt.Sprintf("migrations_check_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE DATABASE " + name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() { _, _ = admin.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)") })

	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	return u.String()
}

func open(t *testing.T, dbURL string) *sql.DB {
	t.Helper()
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func queryStrings(t *testing.T, dbURL, query string) []string {
	t.Helper()
	rows, err := open(t, dbURL).Query(query)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func latestMigration(t *testing.T) uint {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no migrations found: %v", err)
	}
	var latest uint
	for _, f := range files {
		var v uint
		if _, err := fmt.Sscanf(filepath.Base(f), "%d_", &v); err == nil && v > latest {
			latest = v
		}
	}
	return latest
}
