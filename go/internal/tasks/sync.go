package tasks

import (
	"database/sql"
	"log"
	"os"
	"strconv"
	"time"

	"emby-analytics/internal/emby"
)

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func StartSyncLoop(db *sql.DB, em *emby.Client) {
	interval := getEnvInt("SYNC_INTERVAL", 60) // seconds

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		runSync(db, em)
		<-ticker.C
	}
}

func runSync(db *sql.DB, em *emby.Client) {
	// Step 1: sync active sessions
	sessions, err := em.GetActiveSessions()
	if err != nil {
		log.Println("sync error:", err)
	} else {
		for _, s := range sessions {
			upsertUserAndItem(db, s.UserID, s.UserName, s.ItemID, "", "")
			insertPlayEvent(db, s.UserID, s.ItemID, s.PosMs)
		}
	}

	// Step 2: backfill from history for each known user
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
		history, err := em.GetUserPlayHistory(uid, 2) // last 2 days
		if err != nil {
			log.Printf("history error for %s: %v\n", uid, err)
			continue
		}
		for _, h := range history {
			upsertUserAndItem(db, uid, uname, h.Id, h.Name, h.Type)
			posMs := h.PlaybackPos / 10000 // ticks to ms
			insertPlayEvent(db, uid, h.Id, posMs)
		}
	}
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

func insertPlayEvent(db *sql.DB, userID, itemID string, posMs int64) {
	ts := time.Now().UnixMilli()
	_, _ = db.Exec(`INSERT INTO play_event (ts, user_id, item_id, pos_ms)
	                VALUES (?, ?, ?, ?)`,
		ts, userID, itemID, posMs)
	_, _ = db.Exec(`INSERT INTO lifetime_watch (user_id, total_ms)
	                VALUES (?, ?)
	                ON CONFLICT(user_id) DO UPDATE SET
	                    total_ms = total_ms + excluded.total_ms`,
		userID, posMs)
}
