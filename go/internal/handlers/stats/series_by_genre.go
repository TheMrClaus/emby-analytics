package stats

import (
    "database/sql"

    "github.com/gofiber/fiber/v3"
)

type SeriesRow struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

type SeriesByGenreResponse struct {
    Items    []SeriesRow `json:"items"`
    Total    int         `json:"total"`
    Genre    string      `json:"genre"`
    Page     int         `json:"page"`
    PageSize int         `json:"page_size"`
}

// SeriesByGenre lists distinct series that contain the given genre on any episode (after backfill).
func SeriesByGenre(db *sql.DB) fiber.Handler {
    return func(c fiber.Ctx) error {
        genre := c.Params("genre")
        if genre == "" {
            return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "genre parameter is required"})
        }

        page := parseQueryInt(c, "page", 1)
        if page < 1 { page = 1 }
        pageSize := parseQueryInt(c, "page_size", 50)
        if pageSize < 1 || pageSize > 500 { pageSize = 50 }

        // Token-boundary, case-insensitive match against normalized CSV
        cond := "media_type = 'Episode' AND " + excludeLiveTvFilter() + " AND genres IS NOT NULL AND genres != '' AND COALESCE(series_id,'') != '' AND INSTR(LOWER(',' || REPLACE(genres, ', ', ',') || ','), LOWER(',' || ? || ',')) > 0"

        // Count distinct series
        var total int
        if err := db.QueryRow("SELECT COUNT(DISTINCT series_id) FROM library_item WHERE "+cond, genre).Scan(&total); err != nil {
            return c.Status(500).JSON(fiber.Map{"error": err.Error()})
        }

        // Page items
        offset := (page - 1) * pageSize
        q := `
            SELECT series_id, MAX(series_name) as name
            FROM library_item
            WHERE ` + cond + `
            GROUP BY series_id
            ORDER BY name ASC
            LIMIT ? OFFSET ?`
        rows, err := db.Query(q, genre, pageSize, offset)
        if err != nil {
            return c.Status(500).JSON(fiber.Map{"error": err.Error()})
        }
        defer rows.Close()

        out := []SeriesRow{}
        for rows.Next() {
            var r SeriesRow
            if err := rows.Scan(&r.ID, &r.Name); err != nil {
                return c.Status(500).JSON(fiber.Map{"error": err.Error()})
            }
            out = append(out, r)
        }
        if err := rows.Err(); err != nil {
            return c.Status(500).JSON(fiber.Map{"error": err.Error()})
        }

        return c.JSON(SeriesByGenreResponse{
            Items: out,
            Total: total,
            Genre: genre,
            Page: page,
            PageSize: pageSize,
        })
    }
}

