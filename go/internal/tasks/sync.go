package tasks

import (
	"database/sql"
	"emby-analytics/internal/logging"
	"fmt"
	"strings"
	"time"

	"emby-analytics/internal/config"
	"emby-analytics/internal/emby"
)

func StartSyncLoop(db *sql.DB, em *emby.Client, cfg config.Config) {
	logging.Debug("starting play sync loop with interval %d seconds", cfg.SyncIntervalSec)

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

	// Step 1: Check if this is the first sync (empty or very little play data)
	var existingEvents int
	db.QueryRow(`SELECT COUNT(*) FROM play_event`).Scan(&existingEvents)

	isFirstSync := existingEvents < 10 // Consider it "first sync" if less than 10 events
	historyDays := cfg.HistoryDays

	if isFirstSync {
		historyDays = 0 // 0 = unlimited history collection
		logging.Debug("First sync detected (%d existing events) - collecting ALL history", existingEvents)
	}

	// Step 2: active sessions
	sessions, err := em.GetActiveSessions()
	apiCalls++ // Count the GetActiveSessions API call
	if err != nil {
		logging.Debug("sync error:", err)
	} else {
		for _, s := range sessions {
			// upsert user and current item
			upsertUserAndItem(db, s.UserID, s.UserName, s.ItemID, s.ItemName, s.ItemType)

			// Convert ticks (100ns) -> ms and clamp to runtime if present
			posMs := int64(0)
			if s.PosTicks > 0 {
				posMs = s.PosTicks / 10_000
			}
			if s.DurationTicks > 0 {
				rtMs := s.DurationTicks / 10_000
				if posMs > rtMs {
					posMs = rtMs
				}
			}
			if posMs < 0 {
				posMs = 0
			}

			if insertPlayEvent(db, s.UserID, s.ItemID, posMs) {
				insertedEvents++
			}
		}
	}

	// Step 3: backfill from user history (use unlimited history on first sync)
	rows, err := db.Query(`SELECT id, name FROM emby_user`)
	if err != nil {
		logging.Debug("sync user list error:", err)
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

		if isFirstSync {
			logging.Debug("Collecting ALL history for user: %s", uname.String)
		}

		history, err := em.GetUserPlayHistory(uid.String, historyDays)
		apiCalls++ // Count each GetUserPlayHistory API call
		if err != nil {
			logging.Debug("history error for %s: %v\n", uid.String, err)
			continue
		}

		userEvents := 0
		for _, h := range history {
			// Enrich library info too
			upsertUserAndItem(db, uid.String, uname.String, h.Id, h.Name, h.Type)

			// PlaybackPositionTicks -> ms
			posMs := int64(0)
			if h.PlaybackPos > 0 {
				posMs = h.PlaybackPos / 10_000
			}
			if posMs < 0 {
				posMs = 0
			}

			// Use historical timestamp if available, otherwise current time
			var eventTime int64
			if h.DatePlayed != "" {
				if playTime, err := time.Parse(time.RFC3339, h.DatePlayed); err == nil {
					eventTime = playTime.UnixMilli()
				} else if playTime, err := time.Parse("2006-01-02T15:04:05", h.DatePlayed); err == nil {
					eventTime = playTime.UnixMilli()
				} else {
					eventTime = time.Now().UnixMilli()
				}
			} else {
				eventTime = time.Now().UnixMilli()
			}

			if insertPlayEventWithTimestamp(db, uid.String, h.Id, posMs, eventTime) {
				insertedEvents++
				userEvents++
			}
		}

		if isFirstSync && userEvents > 0 {
			logging.Debug("User %s: collected %d historical events", uname.String, userEvents)
		}
	}

	duration := time.Since(startTime)
	if insertedEvents > 0 || apiCalls > 1 || isFirstSync {
		logMsg := fmt.Sprintf("[sync] completed in %v: %d API calls, %d users processed, %d play events inserted",
			duration.Round(time.Millisecond), apiCalls, userCount, insertedEvents)
		if isFirstSync {
			logMsg += " (FULL HISTORY COLLECTION)"
		}
		logging.Debug(logMsg)
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
	rowsAffected, _ := res.RowsAffected()
	return rowsAffected > 0
}

func insertPlayEventWithTimestamp(db *sql.DB, userID, itemID string, posMs int64, timestamp int64) bool {
	res, err := db.Exec(`INSERT OR IGNORE INTO play_event (ts, user_id, item_id, pos_ms)
	                VALUES (?, ?, ?, ?)`,
		timestamp, userID, itemID, posMs)
	if err != nil {
		return false
	}
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
