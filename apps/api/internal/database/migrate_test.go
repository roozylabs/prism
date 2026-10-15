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
