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
	log.Printf("[sync] starting play sync loop with interval %d seconds", cfg.SyncIntervalSec)

	ticker := time.NewTicker(time.Duration(cfg.SyncIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		runSync(db, em, cfg)
		<-ticker.C
	}
}

func runSync(db *sql.DB, em *emby.Client, cfg config.Config) {
	insertedEvents := 0
	apiCalls := 0
	startTime := time.Now()

	// Step 1: active sessions
	sessions, err := em.GetActiveSessions()
	apiCalls++ // Count the GetActiveSessions API call

	if err != nil {
		log.Println("sync error:", err)
	} else {
		for _, s := range sessions {
			upsertUserAndItem(db, s.UserID, s.UserName, s.ItemID, "", "")
			// Convert ticks (100ns) -> ms
			// Convert ticks (100ns) -> ms (clamped to runtime if present)
			posMs := int64(posTicks / 10_000)        // ticks to ms
			durMs := int64(s.DurationTicks / 10_000) // ticks to ms (if you store duration)
			//			posMs := s.PosTicks / 10_000
			if s.DurationTicks > 0 && s.PosTicks > s.DurationTicks {
				posMs = s.DurationTicks / 10_000
			}
			if posMs < 0 {
				posMs = 0
			}
			if insertPlayEvent(db, s.UserID, s.ItemID, posMs) {
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

	userCount := 0
	for rows.Next() {
		var uid, uname sql.NullString
		if err := rows.Scan(&uid, &uname); err != nil {
			continue
		}

		// Skip invalid user records
		if !uid.Valid || strings.TrimSpace(uid.String) == "" {
			continue
		}

		userCount++
		history, err := em.GetUserPlayHistory(uid.String, cfg.HistoryDays)
		apiCalls++ // Count each GetUserPlayHistory API call

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

	duration := time.Since(startTime)
	if insertedEvents > 0 || apiCalls > 1 {
		log.Printf("[sync] completed in %v: %d API calls, %d users processed, %d play events inserted",
			duration.Round(time.Millisecond), apiCalls, userCount, insertedEvents)
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

	// NOTE: No longer updating lifetime_watch here - that's handled by user sync
	rowsAffected, _ := res.RowsAffected()
	return rowsAffected > 0
}

func upsertUserAndItem(db *sql.DB, userID, userName, itemID, itemName, itemType string) {
	// Only insert user if userID is valid (not empty)
	if strings.TrimSpace(userID) != "" {
		_, _ = db.Exec(`INSERT INTO emby_user (id, name) VALUES (?, ?)
		                ON CONFLICT(id) DO UPDATE SET name=excluded.name`,
			userID, userName)
	}

	// Only insert item if itemID is valid (not empty)
	if strings.TrimSpace(itemID) != "" {
		_, _ = db.Exec(`INSERT INTO library_item (id, name, type) VALUES (?, ?, ?)
		                ON CONFLICT(id) DO UPDATE SET
		                    name=COALESCE(excluded.name, library_item.name),
		                    type=COALESCE(excluded.type, library_item.type)`,
			itemID, itemName, itemType)
	}
}

// RunOnce triggers a single sync cycle immediately
func RunOnce(db *sql.DB, em *emby.Client, cfg config.Config) {
	runSync(db, em, cfg)
}
