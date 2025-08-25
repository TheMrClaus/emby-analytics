package admin

import (
	"database/sql"
	"strconv"

	"github.com/gofiber/fiber/v3"
)

// FixPosUnits fixes legacy rows where play_event.pos_ms was stored in Emby ticks (100ns)
// instead of milliseconds. It supports a dry-run (default) and a threshold override.
//
// GET  /admin/fix-pos-units            -> dry-run preview (default threshold 24h)
// POST /admin/fix-pos-units            -> perform the update
// Optional query params:
//
//	?threshold_ms=86400000             -> rows above this are considered "obviously not ms"
//	?dry=true|false                    -> force dry-run or execution
func FixPosUnits(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Defaults: 24h in ms; dry-run for GET, execute for POST
		thresholdStr := c.Query("threshold_ms", "86400000")
		dryStr := c.Query("dry", "")
		method := string(c.Request().Header.Method())

		thresholdMs, err := strconv.ParseInt(thresholdStr, 10, 64)
		if err != nil || thresholdMs <= 0 {
			return c.Status(400).JSON(fiber.Map{"error": "invalid threshold_ms"})
		}

		// Decide dry-run vs execute
		dry := true
		if dryStr != "" {
			// explicit override
			dry = dryStr == "1" || dryStr == "true" || dryStr == "yes"
		} else {
			// default: GET=dry-run, POST=execute
			dry = method != fiber.MethodPost
		}

		// Count candidates
		var cnt int64
		if err := db.QueryRow(`SELECT COUNT(*) FROM play_event WHERE pos_ms > ?`, thresholdMs).Scan(&cnt); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		// Snapshot max before
		var maxBefore sql.NullInt64
		_ = db.QueryRow(`SELECT MAX(pos_ms) FROM play_event`).Scan(&maxBefore)

		if dry || cnt == 0 {
			return c.JSON(fiber.Map{
				"dry_run":        true,
				"threshold_ms":   thresholdMs,
				"would_fix_rows": cnt,
				"max_pos_ms":     maxBefore.Int64,
				"note":           "POST this same endpoint to apply the fix. Only rows with pos_ms > threshold_ms are divided by 10000 (ticks->ms).",
			})
		}

		// Execute the fix (ticks -> ms)
		res, err := db.Exec(`UPDATE play_event SET pos_ms = pos_ms / 10000 WHERE pos_ms > ?`, thresholdMs)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		rows, _ := res.RowsAffected()

		var maxAfter sql.NullInt64
		_ = db.QueryRow(`SELECT MAX(pos_ms) FROM play_event`).Scan(&maxAfter)

		return c.JSON(fiber.Map{
			"dry_run":           false,
			"threshold_ms":      thresholdMs,
			"fixed_rows":        rows,
			"max_pos_ms_before": maxBefore.Int64,
			"max_pos_ms_after":  maxAfter.Int64,
			"message":           "Legacy tick-based pos_ms values normalized to milliseconds.",
		})
	}
}
