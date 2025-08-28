package admin

import (
	"database/sql"
	"log"
	"regexp"
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
			SELECT DISTINCT id, name, media_type 
			FROM library_item 
			WHERE name IS NULL OR name = '' OR name = 'Unknown' 
			   OR media_type IS NULL OR media_type = '' OR media_type = 'Unknown'
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

		// Separate valid GUIDs from invalid IDs
		validGUIDs := make([]string, 0)
		invalidIDs := make([]string, 0)

		// GUID pattern: 8-4-4-4-12 hex digits
		guidPattern := regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

		for _, id := range unknownIDs {
			if guidPattern.MatchString(id) {
				validGUIDs = append(validGUIDs, id)
			} else {
				invalidIDs = append(invalidIDs, id)
				log.Printf("[cleanup] Invalid GUID format: %s", id)
			}
		}

		// Handle invalid IDs (probably orphaned data) - delete if no recent activity
		for _, id := range invalidIDs {
			var recentEvents int
			thirtyDaysAgo := time.Now().AddDate(0, 0, -30).Unix()
			// Updated to use play_intervals table
			db.QueryRow(`SELECT COUNT(*) FROM play_intervals WHERE item_id = ? AND start_ts > ?`,
				id, thirtyDaysAgo).Scan(&recentEvents)

			if recentEvents == 0 {
				// No recent activity - safe to delete
				_, err := db.Exec(`DELETE FROM library_item WHERE id = ?`, id)
				if err == nil {
					result.DeletedItems++
					log.Printf("[cleanup] Deleted invalid ID item %s", id)
				}
			} else {
				result.UnreachableItems++
				log.Printf("[cleanup] Keeping invalid ID item %s (has recent activity)", id)
			}
		}

		// Try to fix valid GUID items via Emby API
		if em != nil && len(validGUIDs) > 0 {
			if embyItems, err := em.ItemsByIDs(validGUIDs); err == nil {
				embyMap := make(map[string]*emby.EmbyItem)
				for _, item := range embyItems {
					embyMap[item.Id] = &item
				}

				// Update items found in Emby
				for _, id := range validGUIDs {
					if embyItem, found := embyMap[id]; found && embyItem.Name != "" && embyItem.Type != "" {
						_, err := db.Exec(`
							UPDATE library_item 
							SET name = ?, media_type = ? 
							WHERE id = ?
						`, embyItem.Name, embyItem.Type, id)
						if err == nil {
							result.FixedItems++
							log.Printf("[cleanup] Fixed item %s: %s (%s)", id, embyItem.Name, embyItem.Type)
						}
					} else {
						// Item not found in Emby (probably deleted)
						var recentEvents int
						thirtyDaysAgo := time.Now().AddDate(0, 0, -30).Unix()
						// Updated to use play_intervals table
						db.QueryRow(`SELECT COUNT(*) FROM play_intervals WHERE item_id = ? AND start_ts > ?`,
							id, thirtyDaysAgo).Scan(&recentEvents)

						if recentEvents == 0 {
							// No recent activity - safe to delete
							_, err := db.Exec(`DELETE FROM library_item WHERE id = ?`, id)
							if err == nil {
								result.DeletedItems++
								log.Printf("[cleanup] Deleted unused GUID item %s", id)
							}
						} else {
							result.UnreachableItems++
							log.Printf("[cleanup] Keeping GUID item %s (has recent activity)", id)
						}
					}
				}
			} else {
				log.Printf("[cleanup] Emby API failed for valid GUIDs: %v", err)
				for range validGUIDs {
					result.UnreachableItems++
				}
			}
		} else {
			if len(validGUIDs) > 0 {
				log.Printf("[cleanup] No Emby client available")
				for range validGUIDs {
					result.UnreachableItems++
				}
			}
		}

		log.Printf("[cleanup] Completed: %d fixed, %d deleted, %d unreachable",
			result.FixedItems, result.DeletedItems, result.UnreachableItems)

		return c.JSON(result)
	}
}
