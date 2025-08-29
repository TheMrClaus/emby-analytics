package stats

import (
	"database/sql"

	"github.com/gofiber/fiber/v3"
)

// PlayMethods returns a breakdown of playback methods over the last N days (default 30).
// Response shape: { "methods": { "DirectPlay": 123, "DirectStream": 45, "Transcode": 67, "Unknown": 3 } }
func PlayMethods(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		days := c.QueryInt("days", 30)

		// We infer a unified "method" per session prioritizing Transcode > DirectStream > DirectPlay.
		// Adjust column names if needed to match your play_sessions schema.
		// Expected columns: video_method, audio_method, start_time.
		q := `
			WITH sessions AS (
				SELECT
					CASE
						WHEN COALESCE(video_method, '') = 'Transcode' OR COALESCE(audio_method, '') = 'Transcode' THEN 'Transcode'
						WHEN COALESCE(video_method, '') = 'DirectStream' OR COALESCE(audio_method, '') = 'DirectStream' THEN 'DirectStream'
						WHEN COALESCE(video_method, '') = 'DirectPlay' OR COALESCE(audio_method, '') = 'DirectPlay' THEN 'DirectPlay'
						ELSE 'Unknown'
					END AS method
				FROM play_sessions
				WHERE start_time >= datetime('now', '-' || ? || ' day')
			)
			SELECT method, COUNT(*) as cnt
			FROM sessions
			GROUP BY method
		`

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
