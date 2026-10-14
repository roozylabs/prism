// Command reencrypt is a one-off migration that brings every stored secret
// under the current ENCRYPTION_KEY. It encrypts values that older releases
// stored as plaintext and re-encrypts values that were encrypted with a legacy
// key (supplied through LEGACY_ENCRYPTION_KEYS, comma-separated).
//
// It runs as a dry run unless -apply is given. All updates happen in a single
// transaction, so a failed run changes nothing.
//
//	DATABASE_URL=... ENCRYPTION_KEY=<new> LEGACY_ENCRYPTION_KEYS=<old1>,<old2> \
//	  go run ./cmd/reencrypt -apply
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/roozylabs/prism/internal/utils"
)

type column struct {
	table string
	name  string
}

// encryptedColumns lists every column written with utils.EncryptAES256GCM.
var encryptedColumns = []column{
	{"credentials", "encrypted_key"},
	{"credentials", "encrypted_metadata"},
	{"tool_backends", "auth_token_encrypted"},
	{"resource_backends", "auth_token_encrypted"},
	{"resource_backends", "connection_string_encrypted"},
	{"mcp_servers", "auth_token_encrypted"},
	{"mcp_servers", "headers_encrypted"},
}

func main() {
	apply := flag.Bool("apply", false, "write changes (default is a dry run)")
	reactivate := flag.Bool("reactivate", false, "set credentials marked 'invalid' back to 'active' when their key was migrated")
	flag.Parse()

	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	currentKey := os.Getenv("ENCRYPTION_KEY")
	if err := utils.ValidateEncryptionKey(currentKey); err != nil {
		log.Fatalf("refusing to migrate onto this key: %v", err)
	}
	var legacyKeys []string
	for _, k := range strings.Split(os.Getenv("LEGACY_ENCRYPTION_KEYS"), ",") {
		if k = strings.TrimSpace(k); k != "" && k != currentKey {
			legacyKeys = append(legacyKeys, k)
		}
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Fatalf("begin transaction: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	totals := map[utils.ReencryptOutcome]int{}
	for _, col := range encryptedColumns {
		counts, err := migrateColumn(ctx, tx, col, currentKey, legacyKeys, *reactivate)
		if err != nil {
			log.Fatalf("%s.%s: %v", col.table, col.name, err)
		}
		for o, n := range counts {
			totals[o] += n
		}
	}

	log.Printf("summary: unchanged=%d legacy_key=%d plaintext=%d unrecoverable=%d",
		totals[utils.ReencryptUnchanged], totals[utils.ReencryptFromLegacyKey],
		totals[utils.ReencryptFromPlaintext], totals[utils.ReencryptUnrecoverable])
	if totals[utils.ReencryptUnrecoverable] > 0 {
		log.Printf("unrecoverable values were left untouched; supply the missing key in LEGACY_ENCRYPTION_KEYS or re-enter those secrets")
	}

	if !*apply {
		log.Printf("dry run: no changes written (re-run with -apply)")
		return
	}
	if err := tx.Commit(); err != nil {
		log.Fatalf("commit: %v", err)
	}
	log.Printf("changes committed")
}

func migrateColumn(ctx context.Context, tx *sql.Tx, col column, currentKey string, legacyKeys []string, reactivate bool) (map[utils.ReencryptOutcome]int, error) {
	var exists bool
	err := tx.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = $1 AND column_name = $2)`,
		col.table, col.name).Scan(&exists)
	if err != nil {
		return nil, err
	}
	if !exists {
		log.Printf("%s.%s: column not present, skipping", col.table, col.name)
		return nil, nil
	}

	// Table and column names come from the fixed list above, never from input.
	rows, err := tx.QueryContext(ctx, fmt.Sprintf(
		`SELECT id::text, %[2]s FROM %[1]s WHERE %[2]s IS NOT NULL AND %[2]s <> '' FOR UPDATE`,
		col.table, col.name))
	if err != nil {
		return nil, err
	}
	type update struct{ id, value string }
	var updates []update
	counts := map[utils.ReencryptOutcome]int{}
	for rows.Next() {
		var id, stored string
		if err := rows.Scan(&id, &stored); err != nil {
			_ = rows.Close()
			return nil, err
		}
		value, outcome, err := utils.ReencryptValue(stored, currentKey, legacyKeys)
		if err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("row %s: %w", id, err)
		}
		counts[outcome]++
		switch outcome {
		case utils.ReencryptFromLegacyKey, utils.ReencryptFromPlaintext:
			log.Printf("%s.%s: row %s: %s -> re-encrypted", col.table, col.name, id, outcome)
			updates = append(updates, update{id, value})
		case utils.ReencryptUnrecoverable:
			log.Printf("%s.%s: row %s: no supplied key decrypts this value", col.table, col.name, id)
		}
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, u := range updates {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(
			`UPDATE %s SET %s = $1, updated_at = NOW() WHERE id::text = $2`, col.table, col.name),
			u.value, u.id); err != nil {
			return nil, fmt.Errorf("update row %s: %w", u.id, err)
		}
		if reactivate && col.table == "credentials" {
			if _, err := tx.ExecContext(ctx,
				`UPDATE credentials SET status = 'active' WHERE id::text = $1 AND status = 'invalid'`, u.id); err != nil {
				return nil, fmt.Errorf("reactivate row %s: %w", u.id, err)
			}
		}
	}
	return counts, nil
}
