package stats

import (
	"database/sql"

	"github.com/gofiber/fiber/v3"
)

// MediaTypeCounts holds per-media-type tallies for a given codec.
type MediaTypeCounts struct {
	Movie   int `json:"Movie"`
	Episode int `json:"Episode"`
}
type CodecBuckets struct {
	Codecs map[string]MediaTypeCounts `json:"codecs"`
}

func Codecs(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		limit := parseQueryInt(c, "limit", 0) // 0 = no limit

		// Use real column names and normalize NULLs to "Unknown".
		// Exclude live/TV channel types from the rollup, like before.
		q := `
			SELECT
				COALESCE(li.video_codec, 'Unknown') AS codec,
				COALESCE(li.media_type, 'Unknown') AS media_type,
				COUNT(*) AS count
			FROM library_item li
			WHERE COALESCE(li.media_type, 'Unknown') NOT IN ('TvChannel', 'LiveTv', 'Channel')
			GROUP BY COALESCE(li.video_codec, 'Unknown'),
					COALESCE(li.media_type, 'Unknown')
			ORDER BY COUNT(*) DESC
			`

		var rows *sql.Rows
		var err error
		if limit > 0 && limit <= 100 {
			q = q + " LIMIT ?"
			rows, err = db.Query(q, limit)
		} else {
			rows, err = db.Query(q)
		}
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		codecs := make(map[string]MediaTypeCounts)
		for rows.Next() {
			var codec string
			var mediaType string
			var count int
			if err := rows.Scan(&codec, &mediaType, &count); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}

			bucket := codecs[codec] // zero-value if missing

			switch mediaType {
			case "Movie":
				bucket.Movie += count
			case "Episode":
				bucket.Episode += count
				// other media types are ignored for now
			}

			codecs[codec] = bucket
		}
		if err := rows.Err(); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(CodecBuckets{Codecs: codecs})
	}
}
