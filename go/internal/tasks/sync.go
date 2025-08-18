package tasks

import (
	"database/sql"
	"log"
	"strings"
	"time"

	"emby-analytics/internal/config"
	"emby-analytics/internal/emby"
)

func StartSyncLoop(db *sql.DB, em *emby.Client, cfg config.Config) {
	ticker := time.NewTicker(time.Duration(cfg.SyncIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		runSync(db, em, cfg)
		<-ticker.C
	}
}

func runSync(db *sql.DB, em *emby.Client, cfg config.Config) {
	insertedEvents := 0

	// Step 1: active sessions
	sessions, err := em.GetActiveSessions()
	if err != nil {
		log.Println("sync error:", err)
	} else {
		for _, s := range sessions {
			upsertUserAndItem(db, s.UserID, s.UserName, s.ItemID, "", "")
			if insertPlayEvent(db, s.UserID, s.ItemID, s.PosMs) {
				insertedEvents++
			}
		}
	}

	// Step 2: backfill from history
	rows, err := db.Query(`SELECT id, name FROM emby_user`)
	if err != nil {
		log.Println("sync user list error:", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var uid, uname sql.NullString
		if err := rows.Scan(&uid, &uname); err != nil {
			continue
		}

		// Skip invalid user records
		if !uid.Valid || strings.TrimSpace(uid.String) == "" {
			continue
		}

		history, err := em.GetUserPlayHistory(uid.String, cfg.HistoryDays)
		if err != nil {
			log.Printf("history error for %s: %v\n", uid.String, err)
			continue
		}
		for _, h := range history {
			upsertUserAndItem(db, uid.String, uname.String, h.Id, h.Name, h.Type)
			posMs := h.PlaybackPos / 10000
			if insertPlayEvent(db, uid.String, h.Id, posMs) {
				insertedEvents++
			}
		}
	}

	if insertedEvents > 0 {
		log.Printf("[sync] inserted %d play events\n", insertedEvents)
	}
}

func insertPlayEvent(db *sql.DB, userID, itemID string, posMs int64) bool {
	ts := time.Now().UnixMilli()

	// Insert the play event
	res, err := db.Exec(`INSERT INTO play_event (ts, user_id, item_id, pos_ms)
	                VALUES (?, ?, ?, ?)`,
		ts, userID, itemID, posMs)
	if err != nil {
		return false
	}

	// Only update lifetime_watch if this is actual progress (position > 30 seconds)
	// and limit updates to prevent inflated totals from frequent sync runs
	if posMs > 30000 { // 30 seconds minimum
		// Check if we've updated this user's total recently (within last 5 minutes)
		var lastUpdate sql.NullInt64
		db.QueryRow(`SELECT MAX(ts) FROM play_event WHERE user_id = ? AND ts > ?`,
			userID, ts-300000).Scan(&lastUpdate) // 5 minutes ago

		if !lastUpdate.Valid || ts-lastUpdate.Int64 > 300000 { // 5+ minutes since last update
			// Add a reasonable session duration (max 1 hour per update)
			sessionDuration := int64(3600000) // 1 hour in ms
			if posMs < sessionDuration {
				sessionDuration = posMs / 10 // Use 10% of position as reasonable session time
			}

			_, _ = db.Exec(`INSERT INTO lifetime_watch (user_id, total_ms)
			                VALUES (?, ?)
			                ON CONFLICT(user_id) DO UPDATE SET
			                    total_ms = total_ms + excluded.total_ms`,
				userID, sessionDuration)
		}
	}

	rowsAffected, _ := res.RowsAffected()
	return rowsAffected > 0
}

func upsertUserAndItem(db *sql.DB, userID, userName, itemID, itemName, itemType string) {
	_, _ = db.Exec(`INSERT INTO emby_user (id, name) VALUES (?, ?)
	                ON CONFLICT(id) DO UPDATE SET name=excluded.name`,
		userID, userName)
	_, _ = db.Exec(`INSERT INTO library_item (id, name, type) VALUES (?, ?, ?)
	                ON CONFLICT(id) DO UPDATE SET
	                    name=COALESCE(excluded.name, library_item.name),
	                    type=COALESCE(excluded.type, library_item.type)`,
		itemID, itemName, itemType)
}

// RunOnce triggers a single sync cycle immediately.
func RunOnce(db *sql.DB, em *emby.Client, cfg config.Config) {
	runSync(db, em, cfg)
}
