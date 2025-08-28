package stats

import (
	"database/sql"
	"regexp"

	"github.com/gofiber/fiber/v3"
)

// getQualityLabel now classifies by WIDTH to match the Emby plugin logic.
// It also falls back to DisplayTitle parsing if width is absent.
func getQualityLabel(width sql.NullInt64, displayTitle sql.NullString) string {
	if width.Valid {
		w := int(width.Int64)
		switch {
		case w >= 3841 && w <= 7680:
			return "8K"
		case w >= 1921 && w <= 3840:
			return "4K"
		case w >= 1281 && w <= 1920:
			return "1080p"
		case w >= 1200 && w <= 1280:
			return "720p"
		case w > 0 && w < 1200:
			return "SD"
		default:
			// fall through to title parsing
		}
	}

	// Fallback: infer from DisplayTitle (e.g., "8K", "4K", "2160p", "1080p", "720p", "576p/540p/480p/360p").
	if displayTitle.Valid && displayTitle.String != "" {
		re := regexp.MustCompile(`(?i)\b(8k|4k|2160p|1440p|1080p|720p|576p|540p|480p|360p)\b`)
		if m := re.FindStringSubmatch(displayTitle.String); len(m) > 0 {
			switch m[1] {
			case "8k":
				return "8K"
			case "4k", "2160p":
				return "4K"
			case "1440p":
				// Not a width bucket in the plugin; closest parity is 1080p.
				return "1080p"
			case "1080p":
				return "1080p"
			case "720p":
				return "720p"
			case "576p", "540p", "480p", "360p":
				return "SD"
			}
		}
	}

	return "Resolution Not Available"
}

// Qualities returns counts grouped by quality label using WIDTH from library_item.
func Qualities(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Query tailored to your schema:
		// - table: library_item
		// - columns: width (nullable), display_title (nullable), media_type ('Movie'|'Episode'|NULL)
		q := `
			SELECT
				width,
				display_title,
				COALESCE(media_type, 'Unknown') AS media_type,
				COUNT(*) AS count
			FROM library_item
			WHERE COALESCE(media_type, 'Unknown') IN ('Movie', 'Episode', 'Unknown')
			GROUP BY width, display_title, media_type
		`

		rows, err := db.Query(q)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "query failed",
				"details": err.Error(),
			})
		}
		defer rows.Close()

		// NOTE: MediaTypeCounts is assumed to be defined in this package (same as codecs endpoint).
		// If it isn't, define it as:
		// type MediaTypeCounts struct { Movie int `json:"movie"`; Episode int `json:"episode"` }
		buckets := make(map[string]MediaTypeCounts)

		for rows.Next() {
			var width sql.NullInt64
			var displayTitle sql.NullString
			var mediaType string
			var count int

			if err := rows.Scan(&width, &displayTitle, &mediaType, &count); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error":   "scan failed",
					"details": err.Error(),
				})
			}

			label := getQualityLabel(width, displayTitle)
			b := buckets[label] // zero-value if missing

			switch mediaType {
			case "Movie":
				b.Movie += count
			case "Episode":
				b.Episode += count
			}

			buckets[label] = b
		}

		if err := rows.Err(); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "row iteration failed",
				"details": err.Error(),
			})
		}

		type QualityBuckets struct {
			Buckets map[string]MediaTypeCounts `json:"buckets"`
		}

		return c.JSON(QualityBuckets{Buckets: buckets})
	}
}
