package stats

import (
	"database/sql"
	"fmt"

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
		serverType, serverID := normalizeServerParam(c.Query("server", ""))

		condition := excludeLiveTvFilterAlias("li")
		condition, args := appendServerFilter(condition, "li", serverType, serverID)
		q := fmt.Sprintf(`
			WITH base AS (
				SELECT
					COALESCE(li.video_codec, 'Unknown') AS codec,
					%s AS media_type
				FROM library_item li
				WHERE %s
			)
			SELECT
				codec,
				media_type,
				COUNT(*) AS count
			FROM base
			WHERE media_type IN ('Movie', 'Episode')
			GROUP BY codec, media_type
			ORDER BY count DESC
			`, normalizedMediaTypeExpr("li"), condition)

		var rows *sql.Rows
		var err error
		if limit > 0 && limit <= 100 {
			q = q + " LIMIT ?"
			args = append(args, limit)
			rows, err = db.Query(q, args...)
		} else {
			rows, err = db.Query(q, args...)
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
