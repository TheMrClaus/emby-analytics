package items

import (
	"database/sql"
	"fmt"
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
						// Prefer API name if DB name empty or if it's just the episode title
						name := rec.Name
						if (name == "" || name == it.Name) && it.Name != "" {
							name = it.Name
							rec.Name = name
						}
						// Build display with better fallbacks
						season := it.ParentIndexNumber
						ep := it.IndexNumber
						series := it.SeriesName
						epname := name

						// Handle cases where we have partial data
						if series == "" {
							// Try to get series name from the episode's parent
							rec.Display = epname
							if epname != "" {
								rec.Type = "Episode" // Keep as Episode if no series info
							}
						} else {
							// We have series info, build full display
							epcode := ""
							if season != nil && ep != nil {
								epcode = fmt.Sprintf("S%02dE%02d", *season, *ep)
							}
							if epcode != "" && epname != "" {
								rec.Display = fmt.Sprintf("%s - %s (%s)", series, epname, epcode)
							} else if epname != "" {
								rec.Display = fmt.Sprintf("%s - %s", series, epname)
							} else {
								rec.Display = series
							}
							rec.Type = "Series" // Change type to Series for display
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
				// Ensure we have at least basic display info
				if r.Display == "" {
					if r.Name != "" {
						r.Display = r.Name
					} else {
						r.Display = "Unknown Item"
					}
				}
				if r.Type == "" {
					r.Type = "Unknown"
				}
				out = append(out, r)
			} else {
				// Unknown ID: create a basic record with fallback info
				out = append(out, ItemRow{
					ID:      id,
					Name:    "Unknown Item",
					Type:    "Unknown",
					Display: "Unknown Item",
				})
			}
		}
		return c.JSON(out)
	}
}

// zero-pad to 2 digits
func two(n int) string {
	if n < 0 {
		return "00"
	}
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	if n < 100 {
		d1 := n / 10
		d2 := n % 10
		return string(rune('0'+d1)) + string(rune('0'+d2))
	}
	// For numbers >= 100, just return as string (shouldn't happen for season/episode numbers)
	return string(rune('0'+(n/10)%10)) + string(rune('0'+n%10))
}
