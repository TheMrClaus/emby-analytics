package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	defaultBusyTimeoutMS = 45000
)

func buildDSN(path string) string {
	// Respect explicit DSNs (file:..., :memory:, etc.) while ensuring pragma defaults.
	if path == "" {
		return fmt.Sprintf("file:emby.db?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(%d)&_pragma=synchronous(NORMAL)", defaultBusyTimeoutMS)
	}

	base := path
	if !strings.HasPrefix(base, "file:") && base != ":memory:" {
		base = "file:" + base
	}
	if base == ":memory:" {
		return base
	}

	if !strings.Contains(base, "?_pragma=") && !strings.Contains(base, "&_pragma=") {
		sep := "?"
		if strings.Contains(base, "?") {
			sep = "&"
		}
		base = fmt.Sprintf("%s%s_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(%d)&_pragma=synchronous(NORMAL)", base, sep, defaultBusyTimeoutMS)
	}

	return base
}

var DB *sql.DB

func Open(path string) (*sql.DB, error) {
	dsn := buildDSN(path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite tuning for mixed read/write workloads in a single-node deployment
	// - WAL enables readers during writes
	// - busy_timeout retries briefly on lock contention instead of failing immediately
	// - synchronous=NORMAL is a good balance for WAL
	_, _ = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON; PRAGMA busy_timeout=45000; PRAGMA synchronous=NORMAL;`)
	// Allow a small pool so we can overlap short-lived queries without starving writes.
	// With WAL + busy timeout the driver will wait for the writer to finish instead of
	// returning SQLITE_BUSY immediately.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(time.Hour)
	DB = db
	return db, nil
}
