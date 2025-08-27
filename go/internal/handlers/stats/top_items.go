package stats

import (
	"database/sql"
	"emby-analytics/internal/emby"
	"emby-analytics/internal/queries"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

type TopItem struct {
	ItemID  string  `json:"item_id"`
	Name    string  `json:"name"`
	Type    string  `json:"type"`
	Hours   float64 `json:"hours"`
	Display string  `json:"display"`
}

func TopItems(db *sql.DB, em *emby.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		timeframe := c.Query("timeframe", "14d")
		limit := parseQueryInt(c, "limit", 10)

		if limit <= 0 || limit > 100 {
			limit = 10
		}

		days := parseTimeframeToDays(timeframe)
		now := time.Now().UTC()
		winEnd := now.Unix()
		winStart := now.AddDate(0, 0, -days).Unix()

		if timeframe == "all-time" {
			winStart = 0
			winEnd = now.AddDate(100, 0, 0).Unix()
		}

		// CORRECTED: Pass 'c' directly as the context.
		queryRows, err := queries.TopItemsByWatchSeconds(c, db, winStart, winEnd, limit)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database query failed: " + err.Error()})
		}

		items := make([]TopItem, len(queryRows))
		for i, r := range queryRows {
			items[i] = TopItem{
				ItemID:  r.ItemID,
				Name:    r.Name,
				Type:    r.Type,
				Hours:   r.Hours,
				Display: r.Name,
			}
		}

		episodeIDs := make([]string, 0)
		allEnrichIDs := make([]string, 0)

		for _, item := range items {
			if strings.EqualFold(item.Type, "Episode") {
				episodeIDs = append(episodeIDs, item.ItemID)
			}
		}
		allEnrichIDs = append(allEnrichIDs, episodeIDs...)

		for _, item := range items {
			if (item.Name == "Unknown" || item.Type == "Unknown") && !strings.EqualFold(item.Type, "Episode") {
				allEnrichIDs = append(allEnrichIDs, item.ItemID)
			}
		}

		if len(allEnrichIDs) > 0 && em != nil {
			if embyItems, err := em.ItemsByIDs(allEnrichIDs); err == nil {
				embyMap := make(map[string]*emby.EmbyItem)
				for i := range embyItems {
					embyMap[embyItems[i].Id] = &embyItems[i]
				}

				for i := range items {
					item := &items[i]
					if it, ok := embyMap[item.ItemID]; ok {
						if strings.EqualFold(item.Type, "Episode") || it.Type == "Episode" {
							name := item.Name
							if (name == "" || name == "Unknown" || name == it.Name) && it.Name != "" {
								item.Name = it.Name
							}
							if it.SeriesName != "" {
								epcode := ""
								if it.ParentIndexNumber != nil && it.IndexNumber != nil {
									epcode = fmt.Sprintf("S%02dE%02d", *it.ParentIndexNumber, *it.IndexNumber)
								}
								if epcode != "" && item.Name != "" {
									item.Display = fmt.Sprintf("%s - %s (%s)", it.SeriesName, item.Name, epcode)
								} else {
									item.Display = fmt.Sprintf("%s - %s", it.SeriesName, item.Name)
								}
								item.Type = "Series"
							} else {
								item.Display = item.Name
							}
						} else {
							if it.Name != "" && (item.Name == "Unknown" || item.Name == "") {
								item.Name = it.Name
								item.Display = it.Name
							}
							if it.Type != "" && (item.Type == "Unknown" || item.Type == "") {
								item.Type = it.Type
							}
						}
					} else if item.Name == "Unknown" || item.Type == "Unknown" {
						item.Name = fmt.Sprintf("Deleted Item (%s)", item.ItemID[:8])
						item.Display = fmt.Sprintf("Deleted Item (%s)", item.ItemID[:8])
						item.Type = "Deleted"
					}
				}
			}
		}

		return c.JSON(items)
	}
}
