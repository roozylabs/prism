package database

import (
	"fmt"
	"log"
	"net/url"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// NewPostgres opens the API's connection pool. statementTimeout bounds every
// statement on it (0 leaves the server default), so a slow query releases
// its connection instead of holding one of the 25 until the client gives up.
func NewPostgres(databaseURL string, statementTimeout time.Duration) (*sqlx.DB, error) {
	dsn, err := withStatementTimeout(databaseURL, statementTimeout)
	if err != nil {
		return nil, fmt.Errorf("invalid database URL: %w", err)
	}
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("Connected to PostgreSQL")
	return db, nil
}

// withStatementTimeout adds statement_timeout (milliseconds) to the
// connection URL unless it already sets one. lib/pq passes it to the server
// as a session setting on every connection of the pool.
func withStatementTimeout(databaseURL string, d time.Duration) (string, error) {
	if d <= 0 {
		return databaseURL, nil
	}
	u, err := url.Parse(databaseURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	if q.Get("statement_timeout") != "" {
		return databaseURL, nil
	}
	q.Set("statement_timeout", strconv.FormatInt(d.Milliseconds(), 10))
	u.RawQuery = q.Encode()
	return u.String(), nil
}
