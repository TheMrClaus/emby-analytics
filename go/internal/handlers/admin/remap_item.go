package admin

import (
	"database/sql"
	"encoding/json"

	"emby-analytics/internal/emby"
	"github.com/gofiber/fiber/v3"
)

type remapRequest struct {
	FromID string `json:"from_id"`
	ToID   string `json:"to_id"`
}

// RemapItem reassigns all references from a stale item_id to a valid one.
// GET  /admin/remap-item?from_id=OLD&to_id=NEW   -> dry-run summary
// POST /admin/remap-item (JSON {from_id,to_id})  -> apply remap
func RemapItem(db *sql.DB, em *emby.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		apply := string(c.Request().Header.Method()) == fiber.MethodPost

		var req remapRequest
		if apply {
			if err := json.Unmarshal(c.Body(), &req); err != nil {
				return c.Status(400).JSON(fiber.Map{"error": "invalid JSON body"})
			}
		} else {
			req.FromID = c.Query("from_id", "")
			req.ToID = c.Query("to_id", "")
		}
		if req.FromID == "" || req.ToID == "" || req.FromID == req.ToID {
			return c.Status(400).JSON(fiber.Map{"error": "from_id and to_id required and must differ"})
		}

		// Verify destination exists (either in DB or in Emby and upsert into DB)
		var toName, toType string
		err := db.QueryRow(`SELECT name, media_type FROM library_item WHERE id = ?`, req.ToID).Scan(&toName, &toType)
		if err == sql.ErrNoRows && em != nil {
			// Try Emby and upsert basic record
			if items, e := em.ItemsByIDs([]string{req.ToID}); e == nil && len(items) > 0 {
				it := items[0]
				toName, toType = it.Name, it.Type
				_, _ = db.Exec(`
                    INSERT INTO library_item (id, server_id, item_id, name, media_type, created_at, updated_at)
                    VALUES (?,?,?,?,?,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)
                    ON CONFLICT(id) DO UPDATE SET name=excluded.name, media_type=excluded.media_type, updated_at=CURRENT_TIMESTAMP
                `, it.Id, it.Id, it.Id, it.Name, it.Type)
			}
		}

		// Count references
		var ivCnt, sessCnt int
		_ = db.QueryRow(`SELECT COUNT(*) FROM play_intervals WHERE item_id = ?`, req.FromID).Scan(&ivCnt)
		_ = db.QueryRow(`SELECT COUNT(*) FROM play_sessions WHERE item_id = ?`, req.FromID).Scan(&sessCnt)

		// Optionally apply changes in a transaction
		updatedIv, updatedSess := 0, 0
		deletedItems := 0
		if apply {
			tx, err := db.Begin()
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			// Repoint intervals
			if res, err := tx.Exec(`UPDATE play_intervals SET item_id = ? WHERE item_id = ?`, req.ToID, req.FromID); err == nil {
				if n, _ := res.RowsAffected(); n > 0 {
					updatedIv = int(n)
				}
			} else {
				_ = tx.Rollback()
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}

			// Repoint sessions (keep item_name as-is to preserve display history)
			if res, err := tx.Exec(`UPDATE play_sessions SET item_id = ? WHERE item_id = ?`, req.ToID, req.FromID); err == nil {
				if n, _ := res.RowsAffected(); n > 0 {
					updatedSess = int(n)
				}
			} else {
				_ = tx.Rollback()
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}

			// Safe delete old library_item
			if res, err := tx.Exec(`DELETE FROM library_item WHERE id = ?`, req.FromID); err == nil {
				if n, _ := res.RowsAffected(); n > 0 {
					deletedItems = int(n)
				}
			} else {
				_ = tx.Rollback()
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}

			if err := tx.Commit(); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
		}

		return c.JSON(fiber.Map{
			"from_id":               req.FromID,
			"to_id":                 req.ToID,
			"to_item":               fiber.Map{"name": toName, "type": toType},
			"refs":                  fiber.Map{"play_intervals": ivCnt, "play_sessions": sessCnt},
			"applied":               apply,
			"updated_intervals":     updatedIv,
			"updated_sessions":      updatedSess,
			"deleted_library_items": deletedItems,
		})
	}
}
