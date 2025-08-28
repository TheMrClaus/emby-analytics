package db

import (
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
)

// IMPORTANT: the path is relative to THIS file's directory (go/internal/db).
// Match both up/down files explicitly to avoid "no matching files" during go:embed.
//
//go:embed migrations/*.up.sql migrations/*.down.sql
var migrationsFS embed.FS

// MigrateUp runs all "up" migrations bundled via go:embed.
func MigrateUp(databaseURL string) error {
	if databaseURL == "" {
		return fmt.Errorf("migrator: empty database URL")
	}

	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("migrator: iofs init: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, databaseURL)
	if err != nil {
		return fmt.Errorf("migrator: create: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrator: up: %w", err)
	}
	return nil
}
