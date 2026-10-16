package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirtyConcurrentIndexIsRebuilt(t *testing.T) {
	dbURL := freshDatabase(t)
	dir := t.TempDir()
	for name, sql := range map[string]string{
		"1_table.up.sql":   `CREATE TABLE t (a int); INSERT INTO t VALUES (1), (1);`,
		"1_table.down.sql": `DROP TABLE t;`,
		"2_index.up.sql":   `CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS t_a_key ON t (a);`,
		"2_index.down.sql": `DROP INDEX IF EXISTS t_a_key;`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(sql), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// The duplicate makes the concurrent build fail: version 2 is left dirty
	// with an INVALID t_a_key.
	if err := RunMigrations(dbURL, dir); err == nil {
		t.Fatal("index migration succeeded on duplicate data")
	}
	db := open(t, dbURL)
	if _, err := db.Exec(`DELETE FROM t WHERE ctid <> (SELECT min(ctid) FROM t)`); err != nil {
		t.Fatal(err)
	}

	// The retry used to force the version and let IF NOT EXISTS skip the
	// invalid index, leaving the unique constraint unenforced.
	if err := RunMigrations(dbURL, dir); err != nil {
		t.Fatalf("retry: %v", err)
	}
	var valid bool
	if err := db.QueryRow(`SELECT indisvalid FROM pg_index WHERE indexrelid = 't_a_key'::regclass`).Scan(&valid); err != nil {
		t.Fatal(err)
	}
	if !valid {
		t.Fatal("t_a_key is still invalid after the retry")
	}
	if _, err := db.Exec(`INSERT INTO t VALUES (1)`); err == nil {
		t.Fatal("unique index does not enforce uniqueness")
	}
}
