package admin

import (
	"database/sql"

	"github.com/gofiber/fiber/v3"
)

// POST /admin/recover-intervals
func RecoverIntervalsHandler(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Create intervals from existing sessions that don't have them
		result, err := db.Exec(`
            INSERT INTO play_intervals (session_fk, item_id, user_id, start_ts, end_ts, 
                                       start_pos_ticks, end_pos_ticks, duration_seconds, seeked)
            SELECT 
                ps.id,
                ps.item_id,
                ps.user_id,
                ps.started_at,
                COALESCE(ps.ended_at, ps.started_at + 3600),
                0,
                0,
                COALESCE(ps.ended_at - ps.started_at, 3600),
                0
            FROM play_sessions ps
            LEFT JOIN play_intervals pi ON pi.session_fk = ps.id
            WHERE pi.id IS NULL
            AND ps.started_at IS NOT NULL
        `)

		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		rowsAffected, _ := result.RowsAffected()

		return c.JSON(fiber.Map{
			"recovered_intervals": rowsAffected,
			"message":             "Successfully recovered missing intervals",
		})
	}
}
