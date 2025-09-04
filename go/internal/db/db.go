package db

import (
	"database/sql"
	"time"

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
    _, _ = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON; PRAGMA busy_timeout=10000; PRAGMA synchronous=NORMAL;`)
    // Increase connection pool for better concurrent access
    // SQLite in WAL mode supports multiple concurrent readers
    db.SetMaxOpenConns(10)
    db.SetMaxIdleConns(5)
    db.SetConnMaxLifetime(time.Hour)
    DB = db
    return db, nil
}
