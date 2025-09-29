package db

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

const (
	maxRetryAttempts    = 8
	initialRetryBackoff = 25 * time.Millisecond
)

// IsBusyError returns true when the error represents a transient SQLite busy/locked state.
func IsBusyError(err error) bool {
	if err == nil {
		return false
	}
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		switch sqliteErr.Code() {
		case sqlite3.SQLITE_BUSY, sqlite3.SQLITE_LOCKED, sqlite3.SQLITE_BUSY_SNAPSHOT:
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") || strings.Contains(msg, "database table is locked")
}

// ExecWithRetry executes the statement, retrying a few times if SQLite reports a busy/locked state.
func ExecWithRetry(db *sql.DB, query string, args ...any) (sql.Result, error) {
	var lastErr error
	backoff := initialRetryBackoff
	for attempt := 0; attempt < maxRetryAttempts; attempt++ {
		res, err := db.Exec(query, args...)
		if err == nil {
			return res, nil
		}
		if !IsBusyError(err) {
			return nil, err
		}
		lastErr = err
		time.Sleep(backoff)
		if backoff < 800*time.Millisecond {
			backoff *= 2
		}
	}
	return nil, lastErr
}

// QueryRowWithRetry executes the query and invokes scan with retry semantics for busy errors.
func QueryRowWithRetry(db *sql.DB, query string, args []any, scan func(*sql.Row) error) error {
	var lastErr error
	backoff := initialRetryBackoff
	for attempt := 0; attempt < maxRetryAttempts; attempt++ {
		row := db.QueryRow(query, args...)
		err := scan(row)
		if err == nil || errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if !IsBusyError(err) {
			return err
		}
		lastErr = err
		time.Sleep(backoff)
		if backoff < 400*time.Millisecond {
			backoff *= 2
		}
	}
	return lastErr
}
