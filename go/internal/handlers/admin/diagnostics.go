package admin

import (
	"database/sql"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// MediaFieldCoverage returns counts of how many items have key metadata fields populated.
func MediaFieldCoverage(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		type counts struct{ Total, WithRunTime, WithSize, WithBitrate, WithWidth, WithHeight, WithCodec int }
		// Overall
		var overall counts
		_ = db.QueryRow(`
            SELECT
              COUNT(*) AS total,
              SUM(CASE WHEN COALESCE(run_time_ticks,0) > 0 THEN 1 ELSE 0 END) AS with_runtime,
              SUM(CASE WHEN COALESCE(file_size_bytes,0) > 0 THEN 1 ELSE 0 END) AS with_size,
              SUM(CASE WHEN COALESCE(bitrate_bps,0) > 0 THEN 1 ELSE 0 END) AS with_bitrate,
              SUM(CASE WHEN COALESCE(width,0) > 0 THEN 1 ELSE 0 END) AS with_width,
              SUM(CASE WHEN COALESCE(height,0) > 0 THEN 1 ELSE 0 END) AS with_height,
              SUM(CASE WHEN TRIM(COALESCE(video_codec,'')) <> '' THEN 1 ELSE 0 END) AS with_codec
            FROM library_item
        `).Scan(&overall.Total, &overall.WithRunTime, &overall.WithSize, &overall.WithBitrate, &overall.WithWidth, &overall.WithHeight, &overall.WithCodec)

		// By media_type (Movie/Episode)
		rows, _ := db.Query(`
            SELECT COALESCE(media_type,'Unknown') AS mt,
              COUNT(*) AS total,
              SUM(CASE WHEN COALESCE(run_time_ticks,0) > 0 THEN 1 ELSE 0 END) AS with_runtime,
              SUM(CASE WHEN COALESCE(file_size_bytes,0) > 0 THEN 1 ELSE 0 END) AS with_size,
              SUM(CASE WHEN COALESCE(bitrate_bps,0) > 0 THEN 1 ELSE 0 END) AS with_bitrate,
              SUM(CASE WHEN COALESCE(width,0) > 0 THEN 1 ELSE 0 END) AS with_width,
              SUM(CASE WHEN COALESCE(height,0) > 0 THEN 1 ELSE 0 END) AS with_height,
              SUM(CASE WHEN TRIM(COALESCE(video_codec,'')) <> '' THEN 1 ELSE 0 END) AS with_codec
            FROM library_item
            GROUP BY mt
        `)
		defer func() {
			if rows != nil {
				rows.Close()
			}
		}()
		byType := map[string]counts{}
		for rows != nil && rows.Next() {
			var mt string
			var cc counts
			_ = rows.Scan(&mt, &cc.Total, &cc.WithRunTime, &cc.WithSize, &cc.WithBitrate, &cc.WithWidth, &cc.WithHeight, &cc.WithCodec)
			byType[mt] = cc
		}

		return c.JSON(fiber.Map{
			"overall": overall,
			"by_type": byType,
		})
	}
}

// MissingItems lists items missing a particular field.
// query params: field=run_time_ticks|file_size_bytes|bitrate_bps|width|height|video_codec, media_type=Movie|Episode, limit=50
func MissingItems(db *sql.DB) fiber.Handler {
	validFields := map[string]bool{
		"run_time_ticks":  true,
		"file_size_bytes": true,
		"bitrate_bps":     true,
		"width":           true,
		"height":          true,
		"video_codec":     true,
	}
	return func(c fiber.Ctx) error {
		field := strings.ToLower(string(c.Query("field")))
		if !validFields[field] {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid field"})
		}
		mediaType := c.Query("media_type")
		limit := 50
		if sv := string(c.Query("limit")); sv != "" {
			if v, err := strconv.Atoi(sv); err == nil && v > 0 && v <= 500 {
				limit = v
			}
		}

		where := "(COALESCE(" + field + ", 0) = 0 OR " + field + " IS NULL)"
		args := []any{}
		if mediaType != "" {
			where += " AND media_type = ?"
			args = append(args, mediaType)
		}
		q := "SELECT id, name, media_type, created_at, width, height, video_codec, run_time_ticks, file_size_bytes, bitrate_bps FROM library_item WHERE " + where + " ORDER BY created_at DESC LIMIT ?"
		args = append(args, limit)

		rows, err := db.Query(q, args...)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()
		type row struct {
			ID            string  `json:"id"`
			Name          *string `json:"name"`
			MediaType     *string `json:"media_type"`
			CreatedAt     *string `json:"created_at"`
			Width         *int    `json:"width"`
			Height        *int    `json:"height"`
			VideoCodec    *string `json:"video_codec"`
			RunTimeTicks  *int64  `json:"run_time_ticks"`
			FileSizeBytes *int64  `json:"file_size_bytes"`
			BitrateBps    *int64  `json:"bitrate_bps"`
		}
		out := []row{}
		for rows.Next() {
			var r row
			_ = rows.Scan(&r.ID, &r.Name, &r.MediaType, &r.CreatedAt, &r.Width, &r.Height, &r.VideoCodec, &r.RunTimeTicks, &r.FileSizeBytes, &r.BitrateBps)
			out = append(out, r)
		}
		return c.JSON(fiber.Map{"missing": field, "media_type": mediaType, "limit": limit, "items": out})
	}
}
