package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "modernc.org/sqlite"
)

var DB *sql.DB

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	_, _ = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;`)
	DB = db
	return db, nil
}

func RunMigrations(db *sql.DB, migrationPath string) error {
	driver, err := sqlite.WithInstance(db, &sqlite.Config{})
	if err != nil {
		return fmt.Errorf("could not create sqlite driver: %w", err)
	}

	// Ensure the migration path exists
	if _, err := os.Stat(migrationPath); os.IsNotExist(err) {
		log.Printf("Migration directory not found at %s, skipping migrations.", migrationPath)
		return EnsureLegacySchema(db) // Fallback for old schema if no migrations exist
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://"+migrationPath,
		"sqlite", driver)
	if err != nil {
		return fmt.Errorf("could not create migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	log.Println("Database migrations applied successfully.")
	return nil
}

// EnsureLegacySchema is kept for backward compatibility if migrations folder doesn't exist
func EnsureLegacySchema(db *sql.DB) error {
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
`
	_, err := db.Exec(schema)
	return err
}
