package db

import (
    "embed"
    "fmt"
    "io/fs"
    "path/filepath"
    "regexp"
    "strconv"
    "strings"

    "emby-analytics/internal/logging"

    "github.com/golang-migrate/migrate/v4"
    "github.com/golang-migrate/migrate/v4/source/iofs"

    _ "github.com/golang-migrate/migrate/v4/database/mysql"
    _ "github.com/golang-migrate/migrate/v4/database/postgres"
    _ "github.com/golang-migrate/migrate/v4/database/sqlite"
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

    // Log available migrations from embed for debugging
    maxVer, files := listEmbeddedMigrations()
    logging.Info("Embedded migrations", "count", len(files), "latest", maxVer)
    if len(files) > 0 {
        logging.Debug(strings.Join(files, ", "))
    }

    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        return fmt.Errorf("migrator: up: %w", err)
    }

    if v, d, err := m.Version(); err == nil {
        logging.Info("DB migration version", "version", v, "dirty", d)
    }
    return nil
}

var migRe = regexp.MustCompile(`^(\d+)_.+\.(up|down)\.sql$`)

func listEmbeddedMigrations() (int, []string) {
    entries, err := fs.ReadDir(migrationsFS, "migrations")
    if err != nil {
        return 0, nil
    }
    maxV := 0
    var names []string
    for _, e := range entries {
        name := e.Name()
        if m := migRe.FindStringSubmatch(name); m != nil {
            names = append(names, name)
            if v, err := strconv.Atoi(m[1]); err == nil && v > maxV {
                maxV = v
            }
        }
    }
    // Sort lexicographically for display
    if len(names) > 1 {
        // minimal sort without importing sort
        for i := 0; i < len(names); i++ {
            for j := i + 1; j < len(names); j++ {
                if strings.Compare(names[i], names[j]) > 0 {
                    names[i], names[j] = names[j], names[i]
                }
            }
        }
    }
    _ = filepath.Separator // avoid unused import on some toolchains
    return maxV, names
}
