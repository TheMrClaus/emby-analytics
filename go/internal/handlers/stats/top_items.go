package stats

import (
	"database/sql"
	"fmt"
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

// /stats/top/items?timeframe=all-time&limit=10
func TopItems(db *sql.DB, em *emby.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Get timeframe parameter: all-time, 30d, 14d, 7d, 3d, 1d
		timeframe := c.Query("timeframe", "14d")
		limit := parseQueryInt(c, "limit", 10)

		if limit <= 0 || limit > 100 {
			limit = 10
		}

		var rows *sql.Rows
		var err error

		// Handle "all-time" vs time-windowed with the SAME accurate approach
		if timeframe == "all-time" {
			// Use the same accurate approach as time-windowed queries, but without time restriction
			rows, err = db.Query(`
				SELECT 
					li.id, 
					COALESCE(li.name, 'Unknown') as name, 
					COALESCE(li.type, 'Unknown') as type,
					SUM(max_pos_ms) / 3600000.0 AS hours
				FROM (
					-- Get the max watch position for each user+item combination (all time)
					SELECT
						user_id,
						item_id,
						MAX(pos_ms) as max_pos_ms
					FROM play_event
					WHERE item_id != '' AND pos_ms > 60000  -- 1+ minute sessions
					GROUP BY user_id, item_id
				) AS user_item_max
				LEFT JOIN library_item li ON li.id = user_item_max.item_id
				WHERE li.type NOT IN ('TvChannel', 'LiveTv', 'Channel') 
				GROUP BY li.id, li.name, li.type
				HAVING SUM(max_pos_ms) > 600000  -- At least 10 minutes total
				ORDER BY hours DESC
				LIMIT ?;
			`, limit)
		} else {
			// Handle time-windowed queries with accurate position-based calculation
			days := parseTimeframeToDays(timeframe)
			if days <= 0 {
				days = 14 // fallback
			}

			fromMs := time.Now().AddDate(0, 0, -days).UnixMilli()

			rows, err = db.Query(`
				SELECT 
					li.id, 
					COALESCE(li.name, 'Unknown') as name, 
					COALESCE(li.type, 'Unknown') as type,
					SUM(max_pos_ms) / 3600000.0 AS hours
				FROM (
					-- Get the max watch position for each user+item combination within the time window
					SELECT
						user_id,
						item_id,
						MAX(pos_ms) as max_pos_ms
					FROM play_event
					WHERE ts >= ? AND item_id != '' AND pos_ms > 60000  -- 1+ minute sessions
					GROUP BY user_id, item_id
				) AS user_item_max
				LEFT JOIN library_item li ON li.id = user_item_max.item_id
				WHERE li.type NOT IN ('TvChannel', 'LiveTv', 'Channel') 
				GROUP BY li.id, li.name, li.type
				HAVING SUM(max_pos_ms) > 600000  -- At least 10 minutes total
				ORDER BY hours DESC
				LIMIT ?;
			`, fromMs, limit)
		}

		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		items := make([]TopItem, 0)
		episodeIDs := make([]string, 0)

		for rows.Next() {
			var ti TopItem
			if err := rows.Scan(&ti.ItemID, &ti.Name, &ti.Type, &ti.Hours); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}

			// Set default display to name
			ti.Display = ti.Name
			items = append(items, ti)

			// Collect episode IDs for enrichment
			if strings.EqualFold(ti.Type, "Episode") {
				episodeIDs = append(episodeIDs, ti.ItemID)
			}
		}

		// Also collect IDs for non-episode items that have Unknown names/types
		unknownIDs := make([]string, 0)
		for _, item := range items {
			// Skip episodes (they're handled above) but catch other unknown items
			if !strings.EqualFold(item.Type, "Episode") &&
				(item.Name == "Unknown" || item.Type == "Unknown") {
				unknownIDs = append(unknownIDs, item.ItemID)
			}
		}

		// Enrich Episodes via Emby API
		if len(episodeIDs) > 0 && em != nil {
			if embyItems, err := em.ItemsByIDs(episodeIDs); err == nil {
				// Create map for faster lookup
				embyMap := make(map[string]*emby.EmbyItem)
				for _, it := range embyItems {
					embyMap[it.Id] = &it
				}

				// Update items in place
				for i, item := range items {
					if it, ok := embyMap[item.ItemID]; ok && strings.EqualFold(item.Type, "Episode") {
						// Build enhanced display name
						name := item.Name
						if (name == "" || name == it.Name) && it.Name != "" {
							name = it.Name
							items[i].Name = name
						}

						season := it.ParentIndexNumber
						ep := it.IndexNumber
						series := it.SeriesName
						epname := name

						if series == "" {
							items[i].Display = epname
						} else {
							epcode := ""
							if season != nil && ep != nil {
								epcode = fmt.Sprintf("S%02dE%02d", *season, *ep)
							}
							if epcode != "" && epname != "" {
								items[i].Display = fmt.Sprintf("%s - %s (%s)", series, epname, epcode)
							} else if epname != "" {
								items[i].Display = fmt.Sprintf("%s - %s", series, epname)
							} else {
								items[i].Display = series
							}
							items[i].Type = "Series"
						}
					}
				}
			}
		}

		// Also enrich Unknown non-episode items via Emby API
		if len(unknownIDs) > 0 && em != nil {
			if embyItems, err := em.ItemsByIDs(unknownIDs); err == nil {
				// Create map for faster lookup
				embyMap := make(map[string]*emby.EmbyItem)
				for _, it := range embyItems {
					embyMap[it.Id] = &it
				}

				// Update unknown items in place
				for i, item := range items {
					if item.Name == "Unknown" || item.Type == "Unknown" {
						if it, ok := embyMap[item.ItemID]; ok {
							// Found in Emby - update with real data
							if it.Name != "" {
								items[i].Name = it.Name
								items[i].Display = it.Name
							}
							if it.Type != "" {
								items[i].Type = it.Type
							}
						} else {
							// Not found in Emby - item probably deleted, show ID for identification
							items[i].Name = fmt.Sprintf("Deleted Item (%s)", item.ItemID[:8])
							items[i].Display = fmt.Sprintf("Deleted Item (%s)", item.ItemID[:8])
							items[i].Type = "Deleted"
						}
					}
				}
			} else {
				// Emby API failed - at least show item IDs for identification
				for i, item := range items {
					if item.Name == "Unknown" || item.Type == "Unknown" {
						items[i].Name = fmt.Sprintf("Item (%s)", item.ItemID[:8])
						items[i].Display = fmt.Sprintf("Item (%s)", item.ItemID[:8])
						items[i].Type = "Unavailable"
					}
				}
			}
		} else if len(unknownIDs) > 0 {
			// No Emby client - at least show item IDs for identification
			for i, item := range items {
				if item.Name == "Unknown" || item.Type == "Unknown" {
					items[i].Name = fmt.Sprintf("Item (%s)", item.ItemID[:8])
					items[i].Display = fmt.Sprintf("Item (%s)", item.ItemID[:8])
					items[i].Type = "Unavailable"
				}
			}
		}

		return c.JSON(items)
	}
}
