package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

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

// EnsureBaseSchema guarantees the fundamental tables required for startup exist.
// This is run before migrations to prevent race conditions.
func EnsureBaseSchema(db *sql.DB) error {
	log.Println("Ensuring base schema tables (emby_user, library_item, lifetime_watch) exist...")
	baseSchema := `
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
	_, err := db.Exec(baseSchema)
	if err != nil {
		return fmt.Errorf("failed to ensure base schema: %w", err)
	}
	log.Println("Base schema check complete.")
	return nil
}

func RunMigrations(db *sql.DB, migrationPath string) error {
	driver, err := sqlite.WithInstance(db, &sqlite.Config{})
	if err != nil {
		return fmt.Errorf("could not create sqlite driver instance: %w", err)
	}

	wd, _ := os.Getwd()
	absPath := filepath.Join(wd, migrationPath)
	log.Printf("Attempting to run migrations from path: %s", absPath)

	if _, err := os.Stat(migrationPath); os.IsNotExist(err) {
		log.Printf("MIGRATION ERROR: Directory not found at %s. Analytics tables will not be created.", migrationPath)
		return nil
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://"+migrationPath,
		"sqlite", driver)
	if err != nil {
		return fmt.Errorf("could not create migrate instance: %w", err)
	}

	log.Println("Applying database migrations for analytics tables...")
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	log.Println("Database migrations checked and applied successfully.")
	return nil
}
