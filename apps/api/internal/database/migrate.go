package database

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func RunMigrations(databaseURL, migrationsPath string) error {
	m, err := migrate.New(
		fmt.Sprintf("file://%s", migrationsPath),
		databaseURL,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	// A migration that stopped halfway leaves the version dirty. Every
	// multi-statement file runs in one transaction, so only a single
	// CREATE INDEX CONCURRENTLY can be half done, and it leaves an INVALID
	// index behind that its IF NOT EXISTS would skip on the retry. Drop those
	// indexes, then retry the migration.
	if version, dirty, verErr := m.Version(); verErr == nil && dirty {
		dropped, err := dropInvalidIndexes(databaseURL)
		if err != nil {
			return fmt.Errorf("migration %d is dirty and its invalid indexes could not be dropped: %w", version, err)
		}
		targetVersion := int(version) - 1
		if targetVersion < 0 {
			targetVersion = 0
		}
		log.Printf("Warning: migration %d is dirty; dropped invalid indexes %v and retrying from version %d", version, dropped, targetVersion)
		if err := m.Force(targetVersion); err != nil {
			return fmt.Errorf("force dirty migration %d to %d: %w", version, targetVersion, err)
		}
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Println("Migrations completed successfully")
	return nil
}

// dropInvalidIndexes drops the indexes a failed CREATE INDEX CONCURRENTLY
// left behind. Indexes still being built are also invalid, so those are
// skipped.
func dropInvalidIndexes(databaseURL string) ([]string, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()

	rows, err := db.Query(`
		SELECT i.indexrelid::regclass::text
		FROM pg_index i
		JOIN pg_class c ON c.oid = i.indexrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE NOT i.indisvalid
		  AND n.nspname = current_schema()
		  AND NOT EXISTS (SELECT 1 FROM pg_stat_progress_create_index p WHERE p.index_relid = i.indexrelid)`)
	if err != nil {
		return nil, err
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			_ = rows.Close()
			return nil, err
		}
		names = append(names, name)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, name := range names {
		// regclass output is already quoted where needed.
		if _, err := db.Exec("DROP INDEX CONCURRENTLY IF EXISTS " + name); err != nil {
			return nil, fmt.Errorf("drop %s: %w", name, err)
		}
	}
	return names, nil
}
