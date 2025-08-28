package admin

import (
	"database/sql"
	"strconv"

	"github.com/gofiber/fiber/v3"
)

// FixPosUnits fixes legacy rows where play positions were stored in Emby ticks (100ns)
// instead of seconds/milliseconds. Now works with the play_intervals table.
//
// GET  /admin/fix-pos-units            -> dry-run preview (default threshold 24h)
// POST /admin/fix-pos-units            -> perform the update
// Optional query params:
//
//	?threshold_seconds=86400           -> intervals above this duration are considered "obviously not seconds"
//	?dry=true|false                    -> force dry-run or execution
func FixPosUnits(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Defaults: 24h in seconds; dry-run for GET, execute for POST
		thresholdStr := c.Query("threshold_seconds", "86400")
		dryStr := c.Query("dry", "")
		method := string(c.Request().Header.Method())

		thresholdSeconds, err := strconv.ParseInt(thresholdStr, 10, 64)
		if err != nil || thresholdSeconds <= 0 {
			return c.Status(400).JSON(fiber.Map{"error": "invalid threshold_seconds"})
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

		// Count candidates in play_intervals table
		var cnt int64
		if err := db.QueryRow(`SELECT COUNT(*) FROM play_intervals WHERE duration_seconds > ?`, thresholdSeconds).Scan(&cnt); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		// Also check position values that might be in ticks
		var posCnt int64
		ticksThreshold := thresholdSeconds * 10_000_000 // Convert to ticks (100ns units)
		if err := db.QueryRow(`SELECT COUNT(*) FROM play_intervals WHERE start_pos_ticks > ? OR end_pos_ticks > ?`, ticksThreshold, ticksThreshold).Scan(&posCnt); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		// Snapshot max values before
		var maxDuration, maxStartPos, maxEndPos sql.NullInt64
		_ = db.QueryRow(`SELECT MAX(duration_seconds) FROM play_intervals`).Scan(&maxDuration)
		_ = db.QueryRow(`SELECT MAX(start_pos_ticks) FROM play_intervals`).Scan(&maxStartPos)
		_ = db.QueryRow(`SELECT MAX(end_pos_ticks) FROM play_intervals`).Scan(&maxEndPos)

		if dry || (cnt == 0 && posCnt == 0) {
			return c.JSON(fiber.Map{
				"dry_run":                 true,
				"threshold_seconds":       thresholdSeconds,
				"would_fix_duration_rows": cnt,
				"would_fix_position_rows": posCnt,
				"max_duration_seconds":    maxDuration.Int64,
				"max_start_pos_ticks":     maxStartPos.Int64,
				"max_end_pos_ticks":       maxEndPos.Int64,
				"note":                    "POST this same endpoint to apply the fix. Duration > threshold converted from ticks to seconds, positions > threshold converted from ticks.",
			})
		}

		// Execute the fix
		fixedDuration := int64(0)
		fixedPositions := int64(0)

		// Fix duration_seconds that are clearly in ticks
		if cnt > 0 {
			res, err := db.Exec(`UPDATE play_intervals SET duration_seconds = duration_seconds / 10000000 WHERE duration_seconds > ?`, thresholdSeconds)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "duration fix failed: " + err.Error()})
			}
			fixedDuration, _ = res.RowsAffected()
		}

		// Fix position ticks that are unusually large
		if posCnt > 0 {
			res, err := db.Exec(`
				UPDATE play_intervals 
				SET start_pos_ticks = start_pos_ticks / 10000, 
				    end_pos_ticks = end_pos_ticks / 10000 
				WHERE start_pos_ticks > ? OR end_pos_ticks > ?
			`, ticksThreshold, ticksThreshold)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "position fix failed: " + err.Error()})
			}
			fixedPositions, _ = res.RowsAffected()
		}

		// Get max values after fix
		var maxDurationAfter, maxStartPosAfter, maxEndPosAfter sql.NullInt64
		_ = db.QueryRow(`SELECT MAX(duration_seconds) FROM play_intervals`).Scan(&maxDurationAfter)
		_ = db.QueryRow(`SELECT MAX(start_pos_ticks) FROM play_intervals`).Scan(&maxStartPosAfter)
		_ = db.QueryRow(`SELECT MAX(end_pos_ticks) FROM play_intervals`).Scan(&maxEndPosAfter)

		return c.JSON(fiber.Map{
			"dry_run":              false,
			"threshold_seconds":    thresholdSeconds,
			"fixed_duration_rows":  fixedDuration,
			"fixed_position_rows":  fixedPositions,
			"max_duration_before":  maxDuration.Int64,
			"max_duration_after":   maxDurationAfter.Int64,
			"max_start_pos_before": maxStartPos.Int64,
			"max_start_pos_after":  maxStartPosAfter.Int64,
			"max_end_pos_before":   maxEndPos.Int64,
			"max_end_pos_after":    maxEndPosAfter.Int64,
			"message":              "Legacy tick-based values normalized to proper units.",
		})
	}
}
