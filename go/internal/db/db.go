package db

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

func Open(path string) (*sql.DB, error) {
    db, err := sql.Open("sqlite", path)
    if err != nil {
        return nil, err
    }
    // SQLite tuning for concurrent readers + single writer use case
    // - WAL enables readers during writes
    // - busy_timeout retries briefly on lock contention instead of failing immediately
    // - synchronous=NORMAL is a good balance for WAL
    _, _ = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000; PRAGMA synchronous=NORMAL;`)
    // Limit concurrent connections to avoid SQLITE_BUSY under write load
    db.SetMaxOpenConns(1)
    db.SetMaxIdleConns(1)
    DB = db
    return db, nil
}
