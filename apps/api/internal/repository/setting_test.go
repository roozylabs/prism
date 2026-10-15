package repository

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/roozylabs/prism/internal/database"
)

// migratedDatabase creates a new database next to DATABASE_URL's, applies
// every migration and returns it. The database is dropped when the test ends.
func migratedDatabase(t *testing.T) *sql.DB {
	t.Helper()
	base := os.Getenv("DATABASE_URL")
	if base == "" {
		t.Skip("DATABASE_URL not set")
	}
	admin, err := sql.Open("postgres", base)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	if err := admin.Ping(); err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}

	name := fmt.Sprintf("settings_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE DATABASE " + name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() { _, _ = admin.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)") })

	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	if err := database.RunMigrations(u.String(), "../../migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func settingValues(t *testing.T, r *SettingRepository, orgID string) map[string][2]string {
	t.Helper()
	list, err := r.ListForOrg(context.Background(), orgID)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][2]string{}
	for _, s := range list {
		out[s.Key] = [2]string{s.Value, s.Scope}
	}
	return out
}

func TestSettingsAreScopedToOrganization(t *testing.T) {
	db := migratedDatabase(t)
	ctx := context.Background()
	for _, id := range []string{"org_a", "org_b"} {
		if _, err := db.Exec(`INSERT INTO organizations (id, name, slug) VALUES ($1, $1, $1)`, id); err != nil {
			t.Fatal(err)
		}
	}
	r := NewSettingRepository(db)

	if err := r.SetForOrg(ctx, "org_a", map[string]string{"max_retries": "9", "webhook_url": "https://a.example"}); err != nil {
		t.Fatal(err)
	}

	a := settingValues(t, r, "org_a")
	if a["max_retries"] != [2]string{"9", "organization"} {
		t.Fatalf("org_a max_retries = %v, want its own override", a["max_retries"])
	}
	if a["cooldown_seconds"] != [2]string{"60", "platform"} {
		t.Fatalf("org_a cooldown_seconds = %v, want the platform default", a["cooldown_seconds"])
	}

	b := settingValues(t, r, "org_b")
	if b["max_retries"] != [2]string{"2", "platform"} {
		t.Fatalf("org_b sees max_retries = %v; another organization's override leaked", b["max_retries"])
	}
	if _, ok := b["webhook_url"]; ok {
		t.Fatal("org_b sees org_a's webhook_url")
	}

	if v, err := r.Get(ctx, "max_retries"); err != nil || v != "2" {
		t.Fatalf("platform default max_retries = %q, %v; an organization changed it", v, err)
	}

	// Saving again updates the same override instead of adding a row.
	if err := r.SetForOrg(ctx, "org_a", map[string]string{"max_retries": "4"}); err != nil {
		t.Fatal(err)
	}
	var rows int
	if err := db.QueryRow(`SELECT count(*) FROM settings WHERE org_id = 'org_a' AND key = 'max_retries'`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if got := settingValues(t, r, "org_a")["max_retries"][0]; got != "4" || rows != 1 {
		t.Fatalf("after update: value %q in %d rows, want \"4\" in 1 row", got, rows)
	}
}
