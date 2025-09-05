package admin

import (
    "database/sql"

    "github.com/gofiber/fiber/v3"
)

// POST /admin/cleanup/intervals/superset
// Removes intervals that fully cover other intervals within the same session
// This addresses legacy fallback intervals that spanned the entire session duration.
func CleanupSupersetIntervals(db *sql.DB) fiber.Handler {
    return func(c fiber.Ctx) error {
        res, err := db.Exec(`
            DELETE FROM play_intervals
            WHERE EXISTS (
                SELECT 1 FROM play_intervals p2
                WHERE p2.session_fk = play_intervals.session_fk
                  AND p2.id <> play_intervals.id
                  AND play_intervals.start_ts <= p2.start_ts
                  AND play_intervals.end_ts >= p2.end_ts
            );
        `)
        if err != nil {
            return c.Status(500).JSON(fiber.Map{"error": err.Error()})
        }
        n, _ := res.RowsAffected()
        return c.JSON(fiber.Map{
            "removed_rows": n,
            "message":      "Removed superset intervals (session-spanning fallbacks)",
        })
    }
}
