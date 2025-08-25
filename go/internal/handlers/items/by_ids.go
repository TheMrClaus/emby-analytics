package items

import (
	"database/sql"
	"strings"

	"github.com/gofiber/fiber/v3"

	"emby-analytics/internal/emby"
)

type ItemRow struct {
	ID      string `json:"id"`
	Name    string `json:"name,omitempty"`
	Type    string `json:"type,omitempty"`
	Display string `json:"display,omitempty"`
}

// GET /items/by-ids?ids=a,b,c
func ByIDs(db *sql.DB, em *emby.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		raw := c.Query("ids", "")
		if strings.TrimSpace(raw) == "" {
			return c.JSON([]ItemRow{}) // empty list, not null
		}
		ids := make([]string, 0)
		for _, p := range strings.Split(raw, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				ids = append(ids, p)
			}
		}
		if len(ids) == 0 {
			return c.JSON([]ItemRow{})
		}

		// 1) Get what we already have in SQLite
		placeholders := strings.Repeat("?,", len(ids))
		placeholders = placeholders[:len(placeholders)-1] // drop trailing comma

		args := make([]any, len(ids))
		for i, v := range ids {
			args[i] = v
		}

		rows, err := db.Query(
			`SELECT id, name, type FROM library_item WHERE id IN (`+placeholders+`)`, args...,
		)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		base := make(map[string]ItemRow, len(ids))
		for rows.Next() {
			var r ItemRow
			if err := rows.Scan(&r.ID, &r.Name, &r.Type); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			// Default display = name for non-episodes (or unknown)
			r.Display = r.Name
			base[r.ID] = r
		}

		// 2) Enrich Episodes via Emby API to build nice display strings
		episodeIDs := make([]string, 0)
		for _, id := range ids {
			if rec, ok := base[id]; ok && strings.EqualFold(rec.Type, "Episode") {
				episodeIDs = append(episodeIDs, id)
			}
		}
		if len(episodeIDs) > 0 {
			if em != nil {
				if items, err := em.ItemsByIDs(episodeIDs); err == nil {
					for _, it := range items {
						rec := base[it.Id]
						// Prefer API name if DB name empty
						name := rec.Name
						if name == "" && it.Name != "" {
							name = it.Name
							rec.Name = name
						}
						// Build display
						season := it.ParentIndexNumber
						ep := it.IndexNumber
						series := it.SeriesName
						epname := name
						// Sxx:Eyy or fallback if missing
						epcode := ""
						if season != nil || ep != nil {
							// zero padding like S01E03 (no colon)
							sv := 0
							ev := 0
							if season != nil {
								sv = *season
							}
							if ep != nil {
								ev = *ep
							}
							epcode = "S" + two(sv) + "E" + two(ev)
						}
						if series != "" && epcode != "" && epname != "" {
							rec.Display = series + " - " + epname + " (" + epcode + ")"
						} else if series != "" && epname != "" {
							rec.Display = series + " - " + epname
						} else {
							rec.Display = epname
						}
						// Change type from "Episode" to "Series" for better display
						if rec.Type == "Episode" {
							rec.Type = "Series"
						}
						base[it.Id] = rec
					}
				}
			}
		}

		// 3) Build output in the same order as requested
		out := make([]ItemRow, 0, len(ids))
		for _, id := range ids {
			if r, ok := base[id]; ok {
				out = append(out, r)
			} else {
				// Unknown ID: still return a placeholder record
				out = append(out, ItemRow{ID: id})
			}
		}
		return c.JSON(out)
	}
}

// zero-pad to 2 digits
func two(n int) string {
	switch {
	case n < 0:
		return "00"
	case n < 10:
		return "0" + string('0' + n)[:0] // cheap; replaced below with proper format
	default:
	}
	// simple safe format (avoids fmt import)
	d1 := n / 10
	d2 := n % 10
	return string('0'+d1) + string('0'+d2)
}
