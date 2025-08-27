package admin

import (
	"database/sql"
	"log"
	"time"

	"github.com/gofiber/fiber/v3"

	"emby-analytics/internal/emby"
)

type CleanupResult struct {
	DeletedItems     int `json:"deleted_items"`
	FixedItems       int `json:"fixed_items"`
	UnreachableItems int `json:"unreachable_items"`
}

// CleanupUnknownItems removes or fixes items with missing metadata
func CleanupUnknownItems(db *sql.DB, em *emby.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		log.Println("[cleanup] Starting unknown items cleanup...")

		// Get all unknown/problematic items
		rows, err := db.Query(`
			SELECT DISTINCT id, name, type 
			FROM library_item 
			WHERE name IS NULL OR name = '' OR name = 'Unknown' 
			   OR type IS NULL OR type = '' OR type = 'Unknown'
		`)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		var unknownIDs []string
		itemMap := make(map[string]struct{ Name, Type string })

		for rows.Next() {
			var id, name, itemType sql.NullString
			if err := rows.Scan(&id, &name, &itemType); err != nil {
				continue
			}
			if id.Valid {
				unknownIDs = append(unknownIDs, id.String)
				itemMap[id.String] = struct{ Name, Type string }{
					Name: name.String,
					Type: itemType.String,
				}
			}
		}

		if len(unknownIDs) == 0 {
			return c.JSON(CleanupResult{})
		}

		log.Printf("[cleanup] Found %d unknown items to process", len(unknownIDs))

		result := CleanupResult{}

		// Try to fix items via Emby API
		if em != nil {
			if embyItems, err := em.ItemsByIDs(unknownIDs); err == nil {
				embyMap := make(map[string]*emby.EmbyItem)
				for _, item := range embyItems {
					embyMap[item.Id] = &item
				}

				// Update items found in Emby
				for _, id := range unknownIDs {
					if embyItem, found := embyMap[id]; found && embyItem.Name != "" && embyItem.Type != "" {
						_, err := db.Exec(`
							UPDATE library_item 
							SET name = ?, type = ? 
							WHERE id = ?
						`, embyItem.Name, embyItem.Type, id)
						if err == nil {
							result.FixedItems++
							log.Printf("[cleanup] Fixed item %s: %s (%s)", id, embyItem.Name, embyItem.Type)
						}
					} else {
						// Item not found in Emby - probably deleted
						// Check if it has any play events in last 30 days
						var recentEvents int
						thirtyDaysAgo := time.Now().AddDate(0, 0, -30).UnixMilli()
						db.QueryRow(`SELECT COUNT(*) FROM play_event WHERE item_id = ? AND ts > ?`,
							id, thirtyDaysAgo).Scan(&recentEvents)

						if recentEvents == 0 {
							// No recent activity - safe to delete
							_, err := db.Exec(`DELETE FROM library_item WHERE id = ?`, id)
							if err == nil {
								result.DeletedItems++
								log.Printf("[cleanup] Deleted unused item %s", id)
							}
						} else {
							result.UnreachableItems++
						}
					}
				}
			} else {
				log.Printf("[cleanup] Emby API failed: %v", err)
				result.UnreachableItems = len(unknownIDs)
			}
		} else {
			result.UnreachableItems = len(unknownIDs)
		}

		log.Printf("[cleanup] Completed: %d fixed, %d deleted, %d unreachable",
			result.FixedItems, result.DeletedItems, result.UnreachableItems)

		return c.JSON(result)
	}
}
