package db

import (
	"database/sql"
	"fmt"
	"log"
	"slices"
	"strings"
)

// listTables returns all user tables (excludes SQLite internals).
func listTables(db *sql.DB) ([]string, error) {
	const q = `
SELECT name
FROM sqlite_master
WHERE type='table'
  AND name NOT LIKE 'sqlite_%'
ORDER BY name;`
	rows, err := db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan tables: %w", err)
		}
		out = append(out, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows err: %w", err)
	}
	return out, nil
}

// setDiff returns items in b that are not in a.
func setDiff(a, b []string) []string {
	ma := make(map[string]struct{}, len(a))
	for _, v := range a {
		ma[v] = struct{}{}
	}
	var diff []string
	for _, v := range b {
		if _, ok := ma[v]; !ok {
			diff = append(diff, v)
		}
	}
	slices.Sort(diff)
	return diff
}

// RunMigrationsWithLogging wraps your existing RunMigrations(db, dir)
// and logs each newly created table.
func RunMigrationsWithLogging(db *sql.DB, migrationsDir string, logger *log.Logger) error {
	before, err := listTables(db)
	if err != nil {
		return err
	}

	// Call your existing migration runner. (You already have this.)
	if err := RunMigrations(db, migrationsDir); err != nil {
		return err
	}

	after, err := listTables(db)
	if err != nil {
		return err
	}

	created := setDiff(before, after)
	if len(created) == 0 {
		logger.Println("[migrate] No new tables created.")
		return nil
	}

	// Log each created table (line-by-line) and a compact summary.
	for _, t := range created {
		logger.Printf("[migrate] created table: %s\n", t)
	}
	logger.Printf("[migrate] created tables: %s\n", strings.Join(created, ", "))
	return nil
}

// ListTablesForDebug is a tiny exported helper for startup logs.
// Not required by the logger wrapper; purely optional.
func ListTablesForDebug(db *sql.DB) ([]string, error) {
	return listTables(db)
}
