package cleanup

import (
	"database/sql"
	"fmt"
)

// ItemInfo represents a library item with metadata
type ItemInfo struct {
	ID         string
	Name       string
	MediaType  string
	SeriesName string
}

// FindMatchingItem searches for an existing item that matches the missing item
func FindMatchingItem(db *sql.DB, missingItem ItemInfo) (string, error) {
	var targetID string

	if missingItem.MediaType == "Episode" && missingItem.SeriesName != "" {
		// For episodes with series name, match by series name and episode name
		err := db.QueryRow(`
			SELECT id FROM library_item 
			WHERE series_name = ? AND name = ? AND media_type = 'Episode' AND id != ?
			LIMIT 1
		`, missingItem.SeriesName, missingItem.Name, missingItem.ID).Scan(&targetID)
		if err == sql.ErrNoRows {
			return "", nil
		}
		return targetID, err
	} else if missingItem.MediaType == "Episode" {
		// For episodes without series name (NULL), fall back to matching just by episode name
		err := db.QueryRow(`
			SELECT id FROM library_item 
			WHERE name = ? AND media_type = 'Episode' AND id != ?
			LIMIT 1
		`, missingItem.Name, missingItem.ID).Scan(&targetID)
		if err == sql.ErrNoRows {
			return "", nil
		}
		return targetID, err
	} else {
		// For movies and other types, match by name and type
		err := db.QueryRow(`
			SELECT id FROM library_item 
			WHERE name = ? AND media_type = ? AND id != ?
			LIMIT 1
		`, missingItem.Name, missingItem.MediaType, missingItem.ID).Scan(&targetID)
		if err == sql.ErrNoRows {
			return "", nil
		}
		return targetID, err
	}
}

// MergeItemData merges watch data from fromID to toID using transaction
func MergeItemData(db *sql.DB, fromID, toID string) error {
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

	// Now safely repoint remaining sessions
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
