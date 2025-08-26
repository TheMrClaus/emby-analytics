package stats

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"emby-analytics/internal/emby"
)

type TopItem struct {
	ItemID  string  `json:"item_id"`
	Name    string  `json:"name"`
	Type    string  `json:"type"`
	Hours   float64 `json:"hours"`
	Display string  `json:"display"` // Add enriched display field
}

// /stats/top/items?days=30&limit=10
func TopItems(db *sql.DB, em *emby.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		// accept window=14d|4w, else days=30 as fallback
		days := parseWindowDays(c.Query("window", ""), parseQueryInt(c, "days", 30))
		limit := parseQueryInt(c, "limit", 10)

		if days <= 0 {
			days = 30
		}
		if limit <= 0 || limit > 100 {
			limit = 10
		}

		fromMs := time.Now().AddDate(0, 0, -days).UnixMilli()

		// For top items, we need to aggregate from play_event but use the accurate approach
		// Get items that users have "completed" based on high completion percentage
		rows, err := db.Query(`
			SELECT 
				li.id, 
				COALESCE(li.name, 'Unknown') as name, 
				COALESCE(li.type, 'Unknown') as type,
				COUNT(DISTINCT completed_views.user_id) * 
				CASE 
					WHEN li.type = 'Movie' THEN 2.0    -- Average movie length
					WHEN li.type = 'Episode' THEN 0.75 -- Average episode length  
					ELSE 1.2
				END AS hours
			FROM (
				-- Find users who watched significant portions (90%+) of each item
				SELECT DISTINCT user_id, item_id
				FROM play_event pe1
				WHERE pe1.ts >= ? 
				  AND pe1.pos_ms > (
					SELECT MAX(pe2.pos_ms) * 0.9 
					FROM play_event pe2 
					WHERE pe2.item_id = pe1.item_id
				)
			) completed_views
			LEFT JOIN library_item li ON li.id = completed_views.item_id
			WHERE li.type NOT IN ('TvChannel', 'LiveTv', 'Channel') 
			GROUP BY li.id, li.name, li.type
			ORDER BY hours DESC
			LIMIT ?;
		`, fromMs, limit)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		items := make(map[string]TopItem)
		episodeIDs := make([]string, 0)

		for rows.Next() {
			var ti TopItem
			if err := rows.Scan(&ti.ItemID, &ti.Name, &ti.Type, &ti.Hours); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}

			// Set default display to name
			ti.Display = ti.Name
			items[ti.ItemID] = ti

			// Collect episode IDs for enrichment
			if strings.EqualFold(ti.Type, "Episode") {
				episodeIDs = append(episodeIDs, ti.ItemID)
			}

			log.Printf("Top item: id=%s, name='%s', type='%s', hours=%.2f",
				ti.ItemID, ti.Name, ti.Type, ti.Hours)
		}

		// Enrich Episodes via Emby API to build nice display strings
		if len(episodeIDs) > 0 && em != nil {
			log.Printf("Enriching %d episodes: %v", len(episodeIDs), episodeIDs)
			if embyItems, err := em.ItemsByIDs(episodeIDs); err == nil {
				log.Printf("Emby API returned %d items", len(embyItems))
				for _, it := range embyItems {
					log.Printf("Emby item %s: name='%s', type='%s', series='%s'",
						it.Id, it.Name, it.Type, it.SeriesName)

					if rec, ok := items[it.Id]; ok {
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
						items[it.Id] = rec
					}
				}
			} else {
				log.Printf("Emby API error for episodes: %v", err)
			}
		}

		// Convert map back to slice, preserving order by hours
		out := make([]TopItem, 0, len(items))

		// Re-query with the same logic to preserve order
		rows, err = db.Query(`
			SELECT 
				li.id, 
				COUNT(DISTINCT completed_views.user_id) * 
				CASE 
					WHEN li.type = 'Movie' THEN 2.0
					WHEN li.type = 'Episode' THEN 0.75  
					ELSE 1.2
				END AS hours
			FROM (
				SELECT DISTINCT user_id, item_id
				FROM play_event pe1
				WHERE pe1.ts >= ? 
				  AND pe1.pos_ms > (
					SELECT MAX(pe2.pos_ms) * 0.9 
					FROM play_event pe2 
					WHERE pe2.item_id = pe1.item_id
				)
			) completed_views
			LEFT JOIN library_item li ON li.id = completed_views.item_id
			WHERE li.type NOT IN ('TvChannel', 'LiveTv', 'Channel') 
			GROUP BY li.id, li.name, li.type
			ORDER BY hours DESC
			LIMIT ?;
		`, fromMs, limit)

		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		for rows.Next() {
			var itemID string
			var hours float64
			if err := rows.Scan(&itemID, &hours); err != nil {
				continue
			}
			if item, ok := items[itemID]; ok {
				out = append(out, item)
			}
		}

		return c.JSON(out)
	}
}
