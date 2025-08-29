package stats

import (
	"database/sql"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// hasColumn checks if a table has a column (case-insensitive).
func hasColumn(db *sql.DB, table, col string) bool {
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return false
	}
	defer rows.Close()

	var (
		cid       int
		name      string
		ctype     sql.NullString
		notnull   int
		dfltValue sql.NullString
		pk        int
	)
	for rows.Next() {
		_ = rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk)
		if strings.EqualFold(name, col) {
			return true
		}
	}
	return false
}

// PlayMethods returns a breakdown of playback methods over the last N days (default 30).
// Response: { "methods": { "DirectPlay": N, "DirectStream": N, "Transcode": N, "Unknown": N } }
func PlayMethods(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Fiber v3: parse query param manually
		daysStr := c.Query("days", "30")
		days, err := strconv.Atoi(daysStr)
		if err != nil || days <= 0 {
			days = 30
		}

		// Determine available schema
		hasPlayMethod := hasColumn(db, "play_sessions", "play_method")
		hasVideoMethod := hasColumn(db, "play_sessions", "video_method")
		hasAudioMethod := hasColumn(db, "play_sessions", "audio_method")

		var q string

		switch {
		case hasPlayMethod:
			// Normalize different casings/wordings into the 4 buckets we display.
			q = `
				SELECT 
					CASE
						WHEN LOWER(COALESCE(play_method,'')) IN ('directplay','direct play') THEN 'DirectPlay'
						WHEN LOWER(COALESCE(play_method,'')) IN ('directstream','direct stream') THEN 'DirectStream'
						WHEN LOWER(COALESCE(play_method,'')) IN ('transcode','transcoding') THEN 'Transcode'
						ELSE 'Unknown'
					END AS method,
					COUNT(*) AS cnt
				FROM play_sessions
				WHERE start_time >= datetime('now', '-' || ? || ' day')
				GROUP BY method
			`
		case hasVideoMethod || hasAudioMethod:
			// Build CASE using whichever columns actually exist.
			parts := []string{}
			if hasVideoMethod {
				parts = append(parts, "COALESCE(video_method, '')")
			}
			if hasAudioMethod {
				parts = append(parts, "COALESCE(audio_method, '')")
			}
			joined := strings.Join(parts, " || '|' || ")

			q = `
				WITH sessions AS (
					SELECT
						CASE
							WHEN LOWER(` + joined + `) LIKE '%transcode%' THEN 'Transcode'
							WHEN LOWER(` + joined + `) LIKE '%directstream%' OR LOWER(` + joined + `) LIKE '%direct stream%' THEN 'DirectStream'
							WHEN LOWER(` + joined + `) LIKE '%directplay%' OR LOWER(` + joined + `) LIKE '%direct play%' THEN 'DirectPlay'
							ELSE 'Unknown'
						END AS method
					FROM play_sessions
					WHERE start_time >= datetime('now', '-' || ? || ' day')
				)
				SELECT method, COUNT(*) AS cnt
				FROM sessions
				GROUP BY method
			`
		default:
			// No recognizable columns. Return zeros gracefully.
			return c.JSON(fiber.Map{
				"methods": fiber.Map{
					"DirectPlay":   0,
					"DirectStream": 0,
					"Transcode":    0,
					"Unknown":      0,
				},
				"note": "No play_method / video_method / audio_method columns found in play_sessions",
			})
		}

		rows, err := db.Query(q, days)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		out := map[string]int{
			"DirectPlay":   0,
			"DirectStream": 0,
			"Transcode":    0,
			"Unknown":      0,
		}

		for rows.Next() {
			var method string
			var cnt int
			if err := rows.Scan(&method, &cnt); err == nil {
				out[method] += cnt
			}
		}

		return c.JSON(fiber.Map{"methods": out})
	}
}
