package database

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestWithStatementTimeout(t *testing.T) {
	got, err := withStatementTimeout("postgres://u:p@localhost:5432/db?sslmode=disable", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "statement_timeout=30000") || !strings.Contains(got, "sslmode=disable") {
		t.Fatalf("got %q, want statement_timeout added next to the existing parameters", got)
	}

	// An explicit value in the URL wins.
	in := "postgres://u:p@localhost:5432/db?statement_timeout=5000"
	if got, _ := withStatementTimeout(in, 30*time.Second); got != in {
		t.Fatalf("got %q, want the URL's own statement_timeout kept", got)
	}
	// Zero leaves the server default.
	if got, _ := withStatementTimeout(in[:len(in)-len("?statement_timeout=5000")], 0); strings.Contains(got, "statement_timeout") {
		t.Fatalf("got %q, want no statement_timeout for 0", got)
	}
}

func TestNewPostgresAppliesStatementTimeout(t *testing.T) {
	base := os.Getenv("DATABASE_URL")
	if base == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := NewPostgres(base, 1500*time.Millisecond)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	defer func() { _ = db.Close() }()

	var setting string
	if err := db.QueryRow("SHOW statement_timeout").Scan(&setting); err != nil {
		t.Fatal(err)
	}
	if setting != "1500ms" && setting != "1.5s" {
		t.Fatalf("statement_timeout = %q, want 1500ms on every pooled connection", setting)
	}
	// A statement past the limit is cancelled by the server, not left running.
	if _, err := db.Exec("SELECT pg_sleep(3)"); err == nil || !strings.Contains(err.Error(), "statement timeout") {
		t.Fatalf("pg_sleep(3) err = %v, want a statement timeout", err)
	}
}
