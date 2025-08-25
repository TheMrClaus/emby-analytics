package stats

import (
	"database/sql"

	"github.com/gofiber/fiber/v3"
)

type QualityBuckets struct {
	Buckets map[string]MediaTypeCounts `json:"buckets"`
}

type MediaTypeCounts struct {
	Movie   int `json:"Movie"`
	Episode int `json:"Episode"`
}

func getQualityLabel(height int) string {
	if height >= 2160 {
		return "4K"
	} else if height >= 1080 {
		return "1080p"
	} else if height >= 720 {
		return "720p"
	} else if height > 0 {
		return "SD"
	}
	return "Unknown"
}

func Qualities(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		rows, err := db.Query(`
			SELECT li.height, li.type, COUNT(*) as count
			FROM library_item li
			WHERE li.type NOT IN ('TvChannel', 'LiveTv', 'Channel')
			GROUP BY li.height, li.type
			ORDER BY li.height DESC;
		`)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		buckets := make(map[string]MediaTypeCounts)

		for rows.Next() {
			var height sql.NullInt64
			var mediaType sql.NullString
			var count int
			if err := rows.Scan(&height, &mediaType, &count); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}

			heightVal := 0
			if height.Valid {
				heightVal = int(height.Int64)
			}

			typeVal := "Unknown"
			if mediaType.Valid {
				typeVal = mediaType.String
			}

			quality := getQualityLabel(heightVal)

			if _, exists := buckets[quality]; !exists {
				buckets[quality] = MediaTypeCounts{}
			}

			bucket := buckets[quality]
			if typeVal == "Movie" {
				bucket.Movie += count
			} else if typeVal == "Episode" {
				bucket.Episode += count
			}
			buckets[quality] = bucket
		}

		result := QualityBuckets{Buckets: buckets}
		return c.JSON(result)
	}
}
