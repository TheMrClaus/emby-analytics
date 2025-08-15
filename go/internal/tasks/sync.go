package tasks

import (
	"database/sql"
	"log"
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
		var uid, uname string
		if err := rows.Scan(&uid, &uname); err != nil {
			continue
		}
		history, err := em.GetUserPlayHistory(uid, cfg.HistoryDays)
		if err != nil {
			log.Printf("history error for %s: %v\n", uid, err)
			continue
		}
		for _, h := range history {
			upsertUserAndItem(db, uid, uname, h.Id, h.Name, h.Type)
			posMs := h.PlaybackPos / 10000
			if insertPlayEvent(db, uid, h.Id, posMs) {
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
	res, err := db.Exec(`INSERT INTO play_event (ts, user_id, item_id, pos_ms)
	                VALUES (?, ?, ?, ?)`,
		ts, userID, itemID, posMs)
	if err != nil {
		return false
	}
	_, _ = db.Exec(`INSERT INTO lifetime_watch (user_id, total_ms)
	                VALUES (?, ?)
	                ON CONFLICT(user_id) DO UPDATE SET
	                    total_ms = total_ms + excluded.total_ms`,
		userID, posMs)
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
