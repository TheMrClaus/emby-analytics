package admin

import (
	"database/sql"

	"github.com/gofiber/fiber/v3"
)

type BackfillStartedResult struct {
	UpdatedFromEvents    int64 `json:"updated_from_events"`
	UpdatedFromIntervals int64 `json:"updated_from_intervals"`
	TotalUpdated         int64 `json:"total_updated"`
}

// BackfillStartedAt updates play_sessions.started_at using earliest play_events or play_intervals.
// - First preference: MIN(play_events.created_at)
// - Fallback if no events: MIN(play_intervals.start_ts)
// Only updates when the computed value is earlier than the current started_at.
func BackfillStartedAt(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		var total int64

		// 1) Prefer earliest event timestamp
		res1, err := db.Exec(`
            UPDATE play_sessions AS ps
            SET started_at = (
                SELECT MIN(pe.created_at) FROM play_events pe WHERE pe.session_fk = ps.id
            )
            WHERE EXISTS (SELECT 1 FROM play_events pe WHERE pe.session_fk = ps.id)
              AND started_at > (
                SELECT MIN(pe.created_at) FROM play_events pe WHERE pe.session_fk = ps.id
              )
        `)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		n1, _ := res1.RowsAffected()
		total += n1

		// 2) Fallback to earliest interval start where there are no events
		res2, err := db.Exec(`
            UPDATE play_sessions AS ps
            SET started_at = (
                SELECT MIN(pi.start_ts) FROM play_intervals pi WHERE pi.session_fk = ps.id
            )
            WHERE NOT EXISTS (SELECT 1 FROM play_events pe WHERE pe.session_fk = ps.id)
              AND EXISTS (SELECT 1 FROM play_intervals pi WHERE pi.session_fk = ps.id)
              AND started_at > (
                SELECT MIN(pi.start_ts) FROM play_intervals pi WHERE pi.session_fk = ps.id
              )
        `)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		n2, _ := res2.RowsAffected()
		total += n2

		return c.JSON(BackfillStartedResult{
			UpdatedFromEvents:    n1,
			UpdatedFromIntervals: n2,
			TotalUpdated:         total,
		})
	}
}
