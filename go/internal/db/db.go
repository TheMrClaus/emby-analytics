package db

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	_, _ = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;`)
	return db, nil
}

func EnsureSchema(db *sql.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS emby_user (
    id TEXT PRIMARY KEY,
    name TEXT
);
CREATE TABLE IF NOT EXISTS library_item (
    id TEXT PRIMARY KEY,
    name TEXT,
    type TEXT,
    height INTEGER,
    codec TEXT
);
CREATE TABLE IF NOT EXISTS lifetime_watch (
    user_id TEXT,
    total_ms INTEGER,
    PRIMARY KEY(user_id)
);
CREATE TABLE IF NOT EXISTS play_event (
    ts INTEGER,
    user_id TEXT,
    item_id TEXT,
    pos_ms INTEGER
);
CREATE INDEX IF NOT EXISTS idx_play_event_user ON play_event(user_id);
CREATE INDEX IF NOT EXISTS idx_play_event_item ON play_event(item_id);
CREATE INDEX IF NOT EXISTS idx_play_event_ts ON play_event(ts);
`
	_, err := db.Exec(schema)
	return err
}
