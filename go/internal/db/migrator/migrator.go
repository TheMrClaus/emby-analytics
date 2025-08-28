package migrator

import (
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
)

// IMPORTANT: path is from this file's dir (go/internal/db/migrator) to the migrations dir.
//
//go:embed ../migrations/*.sql
var migFS embed.FS

func Up(databaseURL string) error {
	if databaseURL == "" {
		return fmt.Errorf("migrator: empty database URL")
	}

	src, err := iofs.New(migFS, "../migrations")
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
