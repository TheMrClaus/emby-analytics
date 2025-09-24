package admin

import (
	"database/sql"
	"emby-analytics/internal/logging"
	"strconv"

	"github.com/gofiber/fiber/v3"
)

// FixFallbackIntervals detects and corrects legacy fallback intervals that
// over-count paused wall-clock time by clamping duration to the observed
// position tick delta and to the item's runtime remaining for the session.
//
// POST /admin/cleanup/intervals/fix-fallback?dry_run=true&slack_seconds=120
func FixFallbackIntervals(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		dryRun := c.Query("dry_run", "true") == "true"
		slackSec, _ := strconv.Atoi(c.Query("slack_seconds", "120"))
		if slackSec < 0 {
			slackSec = 0
		}

		// Select candidate intervals where duration_seconds is significantly larger
		// than the position delta (end_pos_ticks - start_pos_ticks)/1e7.
		// We also bring along session start and runtime ticks for clamping.
		const sel = `
            SELECT 
                pi.id,
                pi.session_fk,
                pi.start_ts,
                pi.end_ts,
                pi.duration_seconds,
                pi.start_pos_ticks,
                pi.end_pos_ticks,
                ps.started_at,
                COALESCE(li.run_time_ticks, 0)
            FROM play_intervals pi
            JOIN play_sessions ps ON ps.id = pi.session_fk
            LEFT JOIN library_item li ON li.id = pi.item_id
            WHERE 
                pi.end_pos_ticks > pi.start_pos_ticks
                AND pi.duration_seconds > ((pi.end_pos_ticks - pi.start_pos_ticks) / 10000000) + ?
        `

		type row struct {
			id            int64
			sessionFK     int64
			startTS       int64
			endTS         int64
			durationSec   int64
			startPosTicks int64
			endPosTicks   int64
			sessStartedAt sql.NullInt64
			runTimeTicks  int64
		}

		rows, err := db.Query(sel, slackSec)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "query failed: " + err.Error()})
		}
		defer rows.Close()

		var candidates []row
		for rows.Next() {
			var r row
			if err := rows.Scan(&r.id, &r.sessionFK, &r.startTS, &r.endTS, &r.durationSec, &r.startPosTicks, &r.endPosTicks, &r.sessStartedAt, &r.runTimeTicks); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "scan failed: " + err.Error()})
			}
			candidates = append(candidates, r)
		}
		if err := rows.Err(); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "row err: " + err.Error()})
		}

		updated := 0
		var reducedTotal int64

		tx, err := db.Begin()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "begin tx failed: " + err.Error()})
		}
		defer func() {
			if tx != nil {
				_ = tx.Rollback()
			}
		}()

		for _, r := range candidates {
			posDeltaSec := int64((r.endPosTicks - r.startPosTicks) / 10000000)
			if posDeltaSec <= 0 {
				continue
			}

			// Cap by runtime remaining for this session, if available
			// runtime in seconds
			runtimeSec := int64(r.runTimeTicks / 10000000)
			if runtimeSec > 0 {
				var already sql.NullInt64
				if err := tx.QueryRow(`SELECT COALESCE(SUM(duration_seconds),0) FROM play_intervals WHERE session_fk = ? AND id <> ?`, r.sessionFK, r.id).Scan(&already); err == nil {
					remaining := runtimeSec - already.Int64
					if remaining <= 0 {
						// Nothing left to attribute; clamp to zero length
						posDeltaSec = 0
					} else if posDeltaSec > remaining {
						posDeltaSec = remaining
					}
				}
			}

			// Compute new start_ts anchored to end_ts
			newStart := r.endTS - posDeltaSec
			if r.sessStartedAt.Valid && newStart < r.sessStartedAt.Int64 {
				newStart = r.sessStartedAt.Int64
			}

			if posDeltaSec <= 0 {
				// Set to minimal 0-length? Prefer to reduce rather than delete; skip if zero
				// We'll just skip updating to avoid creating zero/negative intervals
				continue
			}

			if !dryRun {
				if _, err := tx.Exec(`UPDATE play_intervals SET start_ts = ?, duration_seconds = ? WHERE id = ?`, newStart, posDeltaSec, r.id); err != nil {
					logging.Debug("fix-fallback: update failed for id=%d: %v", r.id, err)
					continue
				}
			}
			updated++
			reducedTotal += (r.durationSec - posDeltaSec)
		}

		if !dryRun {
			if err := tx.Commit(); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "commit failed: " + err.Error()})
			}
			tx = nil
		}

		return c.JSON(fiber.Map{
			"dry_run":               dryRun,
			"slack_seconds":         slackSec,
			"candidates":            len(candidates),
			"updated":               updated,
			"total_seconds_reduced": reducedTotal,
		})
	}
}
