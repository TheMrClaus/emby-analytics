package admin

import (
	"database/sql"
	"strconv"

	"github.com/gofiber/fiber/v3"
)

// BackfillPlayMethods updates historical play_sessions rows to derive
// per-stream methods (video_method, audio_method) based on stored
// codec_from/to differences and transcode_reasons. It only updates rows
// within the last N days (default 90) and preserves explicit Transcode
// values.
func BackfillPlayMethods(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		days := 90
		if d := c.Query("days"); d != "" {
			if n, err := strconv.Atoi(d); err == nil && n > 0 {
				days = n
			}
		}

		// Update video_method: respect explicit Transcode, otherwise derive
		vidSQL := `
            UPDATE play_sessions
            SET video_method = (
                CASE 
                    WHEN lower(COALESCE(video_method,'')) = 'transcode' THEN 'Transcode'
                    WHEN COALESCE(video_codec_from,'') <> '' AND COALESCE(video_codec_to,'') <> '' 
                         AND lower(video_codec_from) <> lower(video_codec_to) THEN 'Transcode'
                    WHEN (
                        instr(lower(COALESCE(transcode_reasons,'')), 'subtitle') > 0 OR 
                        instr(lower(COALESCE(transcode_reasons,'')), 'burn') > 0 OR 
                        instr(lower(COALESCE(transcode_reasons,'')), 'video') > 0
                    ) THEN 'Transcode'
                    ELSE 'DirectPlay'
                END
            )
            WHERE started_at >= (strftime('%s','now') - (? * 86400))
              AND (COALESCE(video_method,'') = '' OR lower(video_method) = 'directplay')
        `

		if _, err := db.Exec(vidSQL, days); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "video backfill failed", "details": err.Error()})
		}

		// Update audio_method: respect explicit Transcode, otherwise derive
		audSQL := `
            UPDATE play_sessions
            SET audio_method = (
                CASE 
                    WHEN lower(COALESCE(audio_method,'')) = 'transcode' THEN 'Transcode'
                    WHEN COALESCE(audio_codec_from,'') <> '' AND COALESCE(audio_codec_to,'') <> '' 
                         AND lower(audio_codec_from) <> lower(audio_codec_to) THEN 'Transcode'
                    WHEN instr(lower(COALESCE(transcode_reasons,'')), 'audio') > 0 THEN 'Transcode'
                    ELSE 'DirectPlay'
                END
            )
            WHERE started_at >= (strftime('%s','now') - (? * 86400))
              AND (COALESCE(audio_method,'') = '' OR lower(audio_method) = 'directplay')
        `

		if _, err := db.Exec(audSQL, days); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "audio backfill failed", "details": err.Error()})
		}

		return c.JSON(fiber.Map{"ok": true, "updated_window_days": days})
	}
}
