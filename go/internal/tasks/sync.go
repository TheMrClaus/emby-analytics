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
	sessions, err := em.GetActiveSessions()
	if err != nil {
		log.Println("sync error:", err)
		return
	}

	for _, s := range sessions {
		// Ensure user exists
		_, _ = db.Exec(`INSERT INTO emby_user (id, name) VALUES (?, ?)
		                ON CONFLICT(id) DO UPDATE SET name=excluded.name`,
			s.UserID, s.UserName)

		// Ensure item exists (just ID for now, refresh job fills details)
		_, _ = db.Exec(`INSERT INTO library_item (id) VALUES (?)
		                ON CONFLICT(id) DO NOTHING`, s.ItemID)

		// Insert play event
		ts := time.Now().UnixMilli()
		_, _ = db.Exec(`INSERT INTO play_event (ts, user_id, item_id, pos_ms)
		                VALUES (?, ?, ?, ?)`,
			ts, s.UserID, s.ItemID, s.PosMs)

		// Update lifetime_watch
		_, _ = db.Exec(`INSERT INTO lifetime_watch (user_id, total_ms)
		                VALUES (?, ?)
		                ON CONFLICT(user_id) DO UPDATE SET
		                    total_ms = total_ms + excluded.total_ms`,
			s.UserID, s.PosMs)
	}
}
