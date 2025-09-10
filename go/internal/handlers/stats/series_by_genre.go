package stats

import (
    "database/sql"
    "strings"

    "github.com/gofiber/fiber/v3"
)

type SeriesRow struct {
    ID     string   `json:"id"`
    Name   string   `json:"name"`
    Genres []string `json:"genres,omitempty"`
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
        ids := []string{}
        for rows.Next() {
            var r SeriesRow
            if err := rows.Scan(&r.ID, &r.Name); err != nil {
                return c.Status(500).JSON(fiber.Map{"error": err.Error()})
            }
            out = append(out, r)
            ids = append(ids, r.ID)
        }
        if err := rows.Err(); err != nil {
            return c.Status(500).JSON(fiber.Map{"error": err.Error()})
        }

        // Fetch all genres for the listed series (deduped tokens per series)
        if len(ids) > 0 {
            // Build IN clause placeholders
            placeholders := make([]string, len(ids))
            args := make([]interface{}, 0, len(ids))
            for i, id := range ids {
                placeholders[i] = "?"
                args = append(args, id)
            }
            tokenQuery := `
                WITH RECURSIVE base AS (
                  SELECT series_id, REPLACE(genres, ', ', ',') AS g
                  FROM library_item
                  WHERE media_type = 'Episode' AND ` + excludeLiveTvFilter() + ` AND genres IS NOT NULL AND genres != '' AND series_id IN (` + strings.Join(placeholders, ",") + `)
                ),
                split(series_id, token, rest) AS (
                  SELECT series_id,
                         TRIM(CASE WHEN INSTR(g, ',') = 0 THEN g ELSE SUBSTR(g, 1, INSTR(g, ',') - 1) END),
                         TRIM(CASE WHEN INSTR(g, ',') = 0 THEN '' ELSE SUBSTR(g, INSTR(g, ',') + 1) END)
                  FROM base
                  UNION ALL
                  SELECT series_id,
                         TRIM(CASE WHEN INSTR(rest, ',') = 0 THEN rest ELSE SUBSTR(rest, 1, INSTR(rest, ',') - 1) END),
                         TRIM(CASE WHEN INSTR(rest, ',') = 0 THEN '' ELSE SUBSTR(rest, INSTR(rest, ',') + 1) END)
                  FROM split
                  WHERE rest <> ''
                )
                SELECT series_id, token
                FROM split
                WHERE token IS NOT NULL AND token != ''
                GROUP BY series_id, LOWER(token)
            `
            tr, err := db.Query(tokenQuery, args...)
            if err == nil {
                defer tr.Close()
                // Map series_id -> []genres
                m := map[string][]string{}
                for tr.Next() {
                    var sid, tok string
                    if err := tr.Scan(&sid, &tok); err == nil {
                        m[sid] = append(m[sid], tok)
                    }
                }
                // Attach back to rows
                for i := range out {
                    if g, ok := m[out[i].ID]; ok {
                        out[i].Genres = g
                    }
                }
            }
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
