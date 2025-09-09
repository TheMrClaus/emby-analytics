package admin

import (
    "database/sql"
    "fmt"
    "strings"

    "emby-analytics/internal/audit"
    "emby-analytics/internal/cleanup"
    "emby-analytics/internal/emby"
    "github.com/gofiber/fiber/v3"
)

// CleanupMissingItems scans library_item for IDs that no longer exist in Emby.
// Enhanced version with audit logging and merge capabilities.
// GET  /admin/cleanup/missing-items        -> dry-run summary
// POST /admin/cleanup/missing-items        -> delete safe items, merge items with watch history
// Optional: ?limit=1000 (batch size)
func CleanupMissingItems(db *sql.DB, em *emby.Client) fiber.Handler {
    return func(c fiber.Ctx) error {
        if em == nil {
            return c.Status(500).JSON(fiber.Map{"error": "Emby client not configured"})
        }
        limit := 1000
        if v := c.Query("limit", ""); v != "" {
            if n := parseInt(v); n > 0 { limit = n }
        }
        apply := string(c.Request().Header.Method()) == fiber.MethodPost

        // Initialize audit logger if applying changes
        var logger *audit.CleanupLogger
        var err error
        if apply {
            logger, err = audit.NewCleanupLogger(db, "missing-items", "admin")
            if err != nil {
                return c.Status(500).JSON(fiber.Map{"error": "Failed to initialize audit log: " + err.Error()})
            }
        }

        // Collect candidate IDs with metadata
        rows, err := db.Query(`
            SELECT id, name, media_type, series_name 
            FROM library_item 
            LIMIT ?
        `, limit)
        if err != nil {
            if logger != nil { logger.FailJob(err.Error()) }
            return c.Status(500).JSON(fiber.Map{"error": err.Error()})
        }
        defer rows.Close()
        
        type itemInfo struct {
            ID         string
            Name       string
            MediaType  string
            SeriesName string
        }
        
        items := []itemInfo{}
        ids := []string{}
        for rows.Next() {
            var item itemInfo
            var seriesName sql.NullString
            if err := rows.Scan(&item.ID, &item.Name, &item.MediaType, &seriesName); err == nil {
                if seriesName.Valid { item.SeriesName = seriesName.String }
                items = append(items, item)
                ids = append(ids, item.ID)
            }
        }
        
        if len(ids) == 0 {
            result := fiber.Map{"checked": 0, "missing": 0, "deleted": 0, "merged": 0, "skipped": 0}
            if logger != nil {
                logger.CompleteJob(0, 0, map[string]interface{}{"result": "no_items"})
                result["job_id"] = logger.GetJobID()
            }
            return c.JSON(result)
        }

        // Check existence in Emby in chunks
        chunk := 50
        found := make(map[string]struct{}, len(ids))
        for i := 0; i < len(ids); i += chunk {
            end := i + chunk; if end > len(ids) { end = len(ids) }
            part := ids[i:end]
            embyItems, err := em.ItemsByIDs(part)
            if err != nil {
                if logger != nil { logger.FailJob(err.Error()) }
                return c.Status(500).JSON(fiber.Map{"error": err.Error()})
            }
            for _, it := range embyItems { found[strings.TrimSpace(it.Id)] = struct{}{} }
        }

        // Process missing items
        deleted, merged, skipped := 0, 0, 0
        missingItems := []itemInfo{}
        missingNoIntervals := []itemInfo{}
        
        for _, item := range items {
            if _, ok := found[item.ID]; ok { continue }
            missingItems = append(missingItems, item)
            
            var hasIntervals int
            _ = db.QueryRow(`SELECT 1 FROM play_intervals WHERE item_id = ? LIMIT 1`, item.ID).Scan(&hasIntervals)
            if hasIntervals == 0 {
                missingNoIntervals = append(missingNoIntervals, item)
            }
        }

        if apply {
            // Delete safe items (no watch history)
            for _, item := range missingNoIntervals {
                if _, err := db.Exec(`DELETE FROM library_item WHERE id = ?`, item.ID); err == nil {
                    deleted++
                    logger.LogItemAction("deleted", item.ID, item.Name, item.MediaType, "", 
                        map[string]interface{}{"reason": "no_watch_history"})
                }
            }
            
            // Process items with watch history - try to merge
            missingWithIntervals := []itemInfo{}
            safeToDeleteIDs := make(map[string]struct{}, len(missingNoIntervals))
            for _, item := range missingNoIntervals {
                safeToDeleteIDs[item.ID] = struct{}{}
            }

            for _, item := range missingItems {
                if _, isSafe := safeToDeleteIDs[item.ID]; !isSafe {
                    missingWithIntervals = append(missingWithIntervals, item)
                }
            }
            
            for _, item := range missingWithIntervals {
                targetID, err := cleanup.FindMatchingItem(db, cleanup.ItemInfo(item))
                if err != nil || targetID == "" {
                    skipped++
                    logger.LogItemAction("skipped", item.ID, item.Name, item.MediaType, "", 
                        map[string]interface{}{"reason": "no_matching_target", "error": fmt.Sprintf("%v", err)})
                    continue
                }
                
                // Merge watch data using transaction
                if err := cleanup.MergeItemData(db, item.ID, targetID); err != nil {
                    skipped++
                    logger.LogItemAction("skipped", item.ID, item.Name, item.MediaType, targetID, 
                        map[string]interface{}{"reason": "merge_failed", "error": err.Error()})
                } else {
                    merged++
                    logger.LogItemAction("merged", item.ID, item.Name, item.MediaType, targetID, 
                        map[string]interface{}{"reason": "duplicate_found"})
                }
            }
            
            // Complete audit log
            summary := map[string]interface{}{
                "deleted": deleted,
                "merged": merged, 
                "skipped": skipped,
                "total_missing": len(missingItems),
            }
            logger.CompleteJob(len(ids), deleted+merged, summary)
        }

        result := fiber.Map{
            "checked": len(ids),
            "missing": len(missingItems),
            "missing_no_intervals": len(missingNoIntervals),
            "deleted": deleted,
            "merged": merged,
            "skipped": skipped,
            "applied": apply,
        }
        
        if logger != nil {
            result["job_id"] = logger.GetJobID()
        }
        
        return c.JSON(result)
    }
}

func parseInt(s string) int {
    n := 0
    for _, ch := range s { if ch < '0' || ch > '9' { return n }; n = n*10 + int(ch-'0') }
    return n
}

// findMatchingItem searches for an existing item that matches the missing item
// Uses series name + episode name for episodes, or just name for movies


// mergeItemData merges watch data from fromID to toID using transaction
// Handles UNIQUE constraint on play_sessions(session_id, item_id)
func mergeItemData(db *sql.DB, fromID, toID string) error {
    tx, err := db.Begin()
    if err != nil {
        return err
    }
    
    // Repoint intervals
    if _, err := tx.Exec(`UPDATE play_intervals SET item_id = ? WHERE item_id = ?`, toID, fromID); err != nil {
        tx.Rollback()
        return fmt.Errorf("failed to update play_intervals: %w", err)
    }
    
    // Handle duplicate sessions: delete conflicting sessions from fromID before updating
    // This prevents UNIQUE constraint violation on (session_id, item_id)
    if _, err := tx.Exec(`
        DELETE FROM play_sessions 
        WHERE item_id = ? 
        AND session_id IN (
            SELECT session_id FROM play_sessions WHERE item_id = ?
        )
    `, fromID, toID); err != nil {
        tx.Rollback()
        return fmt.Errorf("failed to clean conflicting sessions: %w", err)
    }
    
    // Now safely repoint remaining sessions (keep item_name as-is to preserve display history)
    if _, err := tx.Exec(`UPDATE play_sessions SET item_id = ? WHERE item_id = ?`, toID, fromID); err != nil {
        tx.Rollback()
        return fmt.Errorf("failed to update play_sessions: %w", err)
    }
    
    // Delete old library_item
    if _, err := tx.Exec(`DELETE FROM library_item WHERE id = ?`, fromID); err != nil {
        tx.Rollback()
        return fmt.Errorf("failed to delete old library_item: %w", err)
    }
    
    return tx.Commit()
}

