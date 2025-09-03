package stats

import (
	"database/sql"
	"emby-analytics/internal/emby"
	"emby-analytics/internal/queries"
	"emby-analytics/internal/tasks"
	"fmt"
	"sort"
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
		timeframe := c.Query("timeframe", "")
		if timeframe == "" {
			// Fallback to days parameter if timeframe not provided
			days := parseQueryInt(c, "days", 14)
			if days <= 0 {
				timeframe = "all-time"
			} else if days == 1 {
				timeframe = "1d"
			} else if days == 3 {
				timeframe = "3d"
			} else if days == 7 {
				timeframe = "7d"
			} else if days == 14 {
				timeframe = "14d"
			} else if days == 30 {
				timeframe = "30d"
			} else {
				timeframe = "30d" // Default for large day values
			}
		}
		if timeframe == "" {
			timeframe = "14d" // Final fallback
		}
		
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

		// 1. Get historical data
		historicalRows, err := queries.TopItemsByWatchSeconds(c, db, winStart, winEnd, 1000)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database query failed: " + err.Error()})
		}

		if err != nil || len(historicalRows) == 0 {
			// Fallback to counting sessions if intervals aren't populated
			rows, err := db.Query(`
        SELECT 
            li.id,
            li.name,
            li.media_type,
            COUNT(DISTINCT ps.id) * 0.5 as hours
        FROM library_item li
        LEFT JOIN play_sessions ps ON ps.item_id = li.id
        WHERE ps.started_at >= ? AND ps.started_at <= ?
        GROUP BY li.id, li.name, li.media_type
        ORDER BY hours DESC
        LIMIT ?
    `, winStart, winEnd, 1000)

			if err == nil {
				defer rows.Close()
				historicalRows = []queries.TopItemRow{}
				for rows.Next() {
					var r queries.TopItemRow
					if err := rows.Scan(&r.ItemID, &r.Name, &r.Type, &r.Hours); err == nil {
						r.Display = r.Name
						historicalRows = append(historicalRows, r)
					}
				}
			}
		}

		// 2. Prepare to combine historical and live data
		combinedHours := make(map[string]float64)
		itemDetails := make(map[string]TopItem)

		for _, row := range historicalRows {
			combinedHours[row.ItemID] += row.Hours
			itemDetails[row.ItemID] = TopItem{ItemID: row.ItemID, Name: row.Name, Type: row.Type}
		}

		// 2.5. Check for missing items from play_intervals that aren't in library_item
		// This handles your specific case where new episodes had watch time but no metadata
		if len(historicalRows) == 0 {
			// Query play_intervals directly to find items with watch time that might be missing from library_item
			intervalRows, err := db.Query(`
				SELECT pi.item_id, SUM(pi.duration_seconds)/3600.0 as hours
				FROM play_intervals pi
				WHERE pi.start_ts >= ? AND pi.start_ts <= ?
				GROUP BY pi.item_id
				HAVING hours > 0
				ORDER BY hours DESC
				LIMIT ?
			`, winStart, winEnd, 1000)

			if err == nil {
				defer intervalRows.Close()
				missingItemIDs := []string{}
				
				for intervalRows.Next() {
					var itemID string
					var hours float64
					if err := intervalRows.Scan(&itemID, &hours); err == nil {
						combinedHours[itemID] += hours
						// Check if we have this item in library_item
						var exists int
						checkErr := db.QueryRow("SELECT 1 FROM library_item WHERE id = ?", itemID).Scan(&exists)
						if checkErr != nil {
							// Item not in library_item, add to missing list
							missingItemIDs = append(missingItemIDs, itemID)
							// Add placeholder details for now
							itemDetails[itemID] = TopItem{ItemID: itemID, Name: "Loading...", Type: "Unknown"}
						}
					}
				}

				// Fetch missing items from Emby in batch
				if len(missingItemIDs) > 0 && em != nil {
					if embyItems, fetchErr := em.ItemsByIDs(missingItemIDs); fetchErr == nil {
						for _, item := range embyItems {
							// Update item details
							itemDetails[item.Id] = TopItem{ItemID: item.Id, Name: item.Name, Type: item.Type}
							// Insert into database for future use
							_, _ = db.Exec(`
								INSERT INTO library_item (id, server_id, item_id, name, media_type, created_at, updated_at)
								VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
								ON CONFLICT(id) DO UPDATE SET
									name = excluded.name,
									media_type = excluded.media_type,
									updated_at = CURRENT_TIMESTAMP
							`, item.Id, item.Id, item.Id, item.Name, item.Type)
						}
					}
				}
			}
		}

		// 3. Get live data and merge
		liveWatchTimes := tasks.GetLiveItemWatchTimes() // Returns seconds
		for itemID, seconds := range liveWatchTimes {
			combinedHours[itemID] += seconds / 3600.0
			// Ensure we have item details, even if it only has a live session
			if _, ok := itemDetails[itemID]; !ok {
				var name, itemType string
				err := db.QueryRow("SELECT name, media_type FROM library_item WHERE id = ?", itemID).Scan(&name, &itemType)
				if err != nil && em != nil {
					// Just-in-time fetch from Emby if not in database
					if embyItems, fetchErr := em.ItemsByIDs([]string{itemID}); fetchErr == nil && len(embyItems) > 0 {
						item := embyItems[0]
						name = item.Name
						itemType = item.Type
						// Insert into database for future use
						_, _ = db.Exec(`
							INSERT INTO library_item (id, server_id, item_id, name, media_type, created_at, updated_at)
							VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
							ON CONFLICT(id) DO UPDATE SET
								name = excluded.name,
								media_type = excluded.media_type,
								updated_at = CURRENT_TIMESTAMP
						`, item.Id, item.Id, item.Id, item.Name, item.Type)
					} else {
						// Fallback for unknown items
						name = fmt.Sprintf("Unknown Item (%s)", itemID[:8])
						itemType = "Unknown"
					}
				}
				itemDetails[itemID] = TopItem{ItemID: itemID, Name: name, Type: itemType}
			}
		}

		// 4. Convert map back to slice
		finalResult := make([]TopItem, 0, len(combinedHours))
		for itemID, hours := range combinedHours {
			details := itemDetails[itemID]
			finalResult = append(finalResult, TopItem{
				ItemID:  itemID,
				Name:    details.Name,
				Type:    details.Type,
				Hours:   hours,
				Display: details.Name, // Default display before enrichment
			})
		}

		// 5. Sort and limit
		sort.Slice(finalResult, func(i, j int) bool {
			return finalResult[i].Hours > finalResult[j].Hours
		})
		if len(finalResult) > limit {
			finalResult = finalResult[:limit]
		}

		// 6. Run your preserved enrichment logic on the final, combined list
		enrichItems(finalResult, em)

		return c.JSON(finalResult)
	}
}

// Your original enrichment logic, now in a helper function for clarity.
func enrichItems(items []TopItem, em *emby.Client) {
    allEnrichIDs := make([]string, 0)
    for _, item := range items {
        if strings.EqualFold(item.Type, "Episode") || item.Name == "Unknown" || item.Type == "Unknown" {
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
                        // For episodes, always use the canonical episode title from Emby
                        if it.Name != "" {
                            item.Name = it.Name
                        }
                        if it.SeriesName != "" {
                            epcode := ""
                            if it.ParentIndexNumber != nil && it.IndexNumber != nil {
                                epcode = fmt.Sprintf("S%02dE%02d", *it.ParentIndexNumber, *it.IndexNumber)
                            }
                            if epcode != "" && item.Name != "" {
                                item.Display = fmt.Sprintf("%s - %s (%s)", it.SeriesName, item.Name, epcode)
                            } else if item.Name != "" {
                                item.Display = fmt.Sprintf("%s - %s", it.SeriesName, item.Name)
                            } else {
                                item.Display = it.SeriesName
                            }
                            // Keep the type as Episode for clarity
                            item.Type = "Episode"
                        } else {
                            // No series name: just show the episode name
                            item.Display = item.Name
                            item.Type = "Episode"
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
}
