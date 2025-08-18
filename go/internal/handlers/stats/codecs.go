package stats

import (
	"database/sql"

	"github.com/gofiber/fiber/v3"
)

type CodecBuckets struct {
	Codecs map[string]MediaTypeCounts `json:"codecs"`
}

func Codecs(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		limit := parseQueryInt(c, "limit", 0) // 0 = no limit

		q := `
			SELECT li.codec, li.type, COUNT(*) as count
			FROM library_item li
			WHERE li.codec IS NOT NULL
			GROUP BY li.codec, li.type
			ORDER BY count DESC
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
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		codecs := make(map[string]MediaTypeCounts)
		for rows.Next() {
			var codec sql.NullString
			var mediaType sql.NullString
			var count int
			if err := rows.Scan(&codec, &mediaType, &count); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}

			codecVal := "Unknown"
			if codec.Valid {
				codecVal = codec.String
			}

			typeVal := "Unknown"
			if mediaType.Valid {
				typeVal = mediaType.String
			}

			if _, exists := codecs[codecVal]; !exists {
				codecs[codecVal] = MediaTypeCounts{}
			}

			bucket := codecs[codecVal]
			if typeVal == "Movie" {
				bucket.Movie += count
			} else if typeVal == "Episode" {
				bucket.Episode += count
			}
			codecs[codecVal] = bucket
		}

		result := CodecBuckets{Codecs: codecs}
		return c.JSON(result)
	}
}
