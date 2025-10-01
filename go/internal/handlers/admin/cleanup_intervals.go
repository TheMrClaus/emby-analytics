package admin

import (
	"database/sql"

	"github.com/gofiber/fiber/v3"
)

// POST /admin/cleanup/intervals/dedupe
// Removes duplicate intervals produced by the old session processor logic
// Keeps the latest row per (session_fk, start_ts) and preserves distinct start_ts
func CleanupDuplicateIntervals(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		res, err := db.Exec(`
            DELETE FROM play_intervals
            WHERE id IN (
                SELECT id FROM (
                    SELECT pi.id
                    FROM play_intervals pi
                    JOIN (
                        SELECT session_fk, start_ts
                        FROM play_intervals
                        GROUP BY session_fk, start_ts
                        HAVING COUNT(*) > 1
                    ) d ON d.session_fk = pi.session_fk AND d.start_ts = pi.start_ts
                    WHERE pi.id NOT IN (
                        SELECT MAX(id)
                        FROM play_intervals p2
                        WHERE p2.session_fk = pi.session_fk AND p2.start_ts = pi.start_ts
                    )
                )
            );
        `)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		n, _ := res.RowsAffected()
		return c.JSON(fiber.Map{
			"removed_rows": n,
			"message":      "Duplicate intervals cleaned (kept latest per session and start time)",
		})
	}
}
